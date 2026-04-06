package event

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

var (
	ErrNotStarted = errors.New("worker pool not started")
	ErrQueueFull  = errors.New("worker queue is full")
)

type WorkerPool struct {
	workers   int
	queueSize int
	queue     chan func(context.Context) error
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	started   bool
	mu        sync.Mutex
}

func NewWorkerPool(workers int, queueSize int) (*WorkerPool, error) {
	if workers <= 0 || queueSize <= 0 {
		return nil, errors.New("invalid worker pool config")
	}
	return &WorkerPool{
		workers:   workers,
		queueSize: queueSize,
		queue:     make(chan func(context.Context) error, queueSize),
	}, nil
}

func (p *WorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil
	}
	if p.queue == nil {
		p.queue = make(chan func(context.Context) error, p.queueSize)
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
				case job, ok := <-p.queue:
					if !ok {
						return
					}
					if err := job(workerCtx); err != nil {
						// TODO: expose job-level retry/backoff/dead-letter policy.
						slog.Error("worker job failed", "err", err)
					}
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
	if job == nil {
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
	case queue <- job:
		return nil
	default:
		// Keep producer non-blocking under pressure; caller decides retry.
		return ErrQueueFull
	}
}
