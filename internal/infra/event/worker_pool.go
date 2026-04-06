package event

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	domain "go-sse-skeleton/internal/domain/event"
	port "go-sse-skeleton/internal/port/event"
)

var (
	ErrNotStarted = errors.New("worker pool not started")
	ErrQueueFull  = errors.New("worker queue is full")
)

type RetryPolicy struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

type WorkerPoolOption func(*WorkerPool)

func WithRetryPolicy(policy RetryPolicy) WorkerPoolOption {
	return func(p *WorkerPool) {
		if policy.InitialBackoff <= 0 {
			policy.InitialBackoff = 100 * time.Millisecond
		}
		if policy.MaxBackoff <= 0 {
			policy.MaxBackoff = 2 * time.Second
		}
		if policy.Multiplier < 1.0 {
			policy.Multiplier = 2.0
		}
		if policy.MaxRetries < 0 {
			policy.MaxRetries = 0
		}
		p.retryPolicy = policy
	}
}

func WithDeadLetterSink(sink port.DeadLetterSink) WorkerPoolOption {
	return func(p *WorkerPool) {
		if sink != nil {
			p.dlq = sink
		}
	}
}

func WithObserver(observer port.Observer) WorkerPoolOption {
	return func(p *WorkerPool) {
		if observer != nil {
			p.observer = observer
		}
	}
}

type jobEnvelope struct {
	event *domain.ChatEvent
	job   func(context.Context) error
}

type WorkerPool struct {
	workers   int
	queueSize int
	queue     chan jobEnvelope
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	started   bool
	mu        sync.Mutex

	retryPolicy RetryPolicy
	dlq         port.DeadLetterSink
	observer    port.Observer
}

func NewWorkerPool(workers int, queueSize int, opts ...WorkerPoolOption) (*WorkerPool, error) {
	if workers <= 0 || queueSize <= 0 {
		return nil, errors.New("invalid worker pool config")
	}
	p := &WorkerPool{
		workers:   workers,
		queueSize: queueSize,
		queue:     make(chan jobEnvelope, queueSize),
		retryPolicy: RetryPolicy{
			MaxRetries:     0,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     2 * time.Second,
			Multiplier:     2.0,
		},
		dlq:      NewNoopDeadLetterSink(),
		observer: NewNoopObserver(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p, nil
}

func (p *WorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil
	}
	if p.queue == nil {
		p.queue = make(chan jobEnvelope, p.queueSize)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.started = true

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case item, ok := <-p.queue:
					if !ok {
						return
					}
					if err := p.executeWithRetry(workerCtx, item); err != nil {
						slog.Error("worker job failed", "err", err)
					}
					p.observer.RecordQueueLen("chat_event", len(p.queue))
				}
			}
		}()
	}
	return nil
}

func (p *WorkerPool) Stop(_ context.Context) error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = false
	cancel := p.cancel
	p.cancel = nil
	close(p.queue)
	p.queue = nil
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	p.wg.Wait()
	return nil
}

func (p *WorkerPool) Submit(ctx context.Context, job func(context.Context) error) error {
	return p.submit(ctx, jobEnvelope{job: job})
}

func (p *WorkerPool) SubmitEvent(ctx context.Context, evt domain.ChatEvent, job func(context.Context) error) error {
	return p.submit(ctx, jobEnvelope{event: &evt, job: job})
}

func (p *WorkerPool) submit(ctx context.Context, item jobEnvelope) error {
	if item.job == nil {
		return errors.New("nil job")
	}
	p.mu.Lock()
	started := p.started
	queue := p.queue
	p.mu.Unlock()
	if !started || queue == nil {
		return ErrNotStarted
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- item:
		p.observer.RecordQueueLen("chat_event", len(queue))
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *WorkerPool) executeWithRetry(ctx context.Context, item jobEnvelope) error {
	var lastErr error
	totalAttempts := p.retryPolicy.MaxRetries + 1
	if totalAttempts <= 0 {
		totalAttempts = 1
	}
	for attempt := 1; attempt <= totalAttempts; attempt++ {
		if err := item.job(ctx); err == nil {
			return nil
		} else {
			lastErr = err
			p.observer.RecordFailed("chat_event", "consumer")
		}
		if attempt == totalAttempts {
			break
		}
		if err := p.waitBackoff(ctx, attempt); err != nil {
			lastErr = err
			break
		}
	}

	if item.event != nil {
		reason := "unknown"
		if lastErr != nil {
			reason = lastErr.Error()
		}
		_ = p.dlq.Push(ctx, *item.event, reason, totalAttempts)
	}
	return lastErr
}

func (p *WorkerPool) waitBackoff(ctx context.Context, attempt int) error {
	delay := p.retryPolicy.InitialBackoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * p.retryPolicy.Multiplier)
		if delay > p.retryPolicy.MaxBackoff {
			delay = p.retryPolicy.MaxBackoff
			break
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
