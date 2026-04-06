package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/event"
)

type testObserver struct {
	mu        sync.Mutex
	failed    int
	queueLens []int
}

func (o *testObserver) RecordPublished(string) {}

func (o *testObserver) RecordConsumed(string, time.Duration) {}

func (o *testObserver) RecordFailed(string, string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failed++
}

func (o *testObserver) RecordQueueLen(string, length int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.queueLens = append(o.queueLens, length)
}

func TestWorkerPoolRetryThenDLQ(t *testing.T) {
	t.Parallel()

	dlq := NewInMemoryDeadLetterSink()
	obs := &testObserver{}
	pool, err := NewWorkerPool(1, 8,
		WithRetryPolicy(RetryPolicy{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond, Multiplier: 2}),
		WithDeadLetterSink(dlq),
		WithObserver(obs),
	)
	if err != nil {
		t.Fatalf("new worker pool: %v", err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatalf("start worker pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop(context.Background()) })

	var attempts int32
	evt := domain.ChatEvent{EventID: "evt-1", AgentID: "a1", SessionID: "s1"}
	submitErr := pool.SubmitEvent(context.Background(), evt, func(context.Context) error {
		atomic.AddInt32(&attempts, 1)
		return errors.New("runner failed")
	})
	if submitErr != nil {
		t.Fatalf("submit event: %v", submitErr)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(dlq.Records()) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("unexpected attempts: got %d want 3", got)
	}
	records := dlq.Records()
	if len(records) != 1 || records[0].Event.EventID != "evt-1" || records[0].Attempts != 3 {
		t.Fatalf("unexpected dlq records: %+v", records)
	}
	if obs.failed < 3 {
		t.Fatalf("expected failed metrics >=3, got %d", obs.failed)
	}
	if len(obs.queueLens) == 0 {
		t.Fatal("expected queue length metric records")
	}
}

func TestWorkerPoolRetryEventuallySuccess(t *testing.T) {
	t.Parallel()

	dlq := NewInMemoryDeadLetterSink()
	pool, err := NewWorkerPool(1, 8,
		WithRetryPolicy(RetryPolicy{MaxRetries: 3, InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond, Multiplier: 2}),
		WithDeadLetterSink(dlq),
	)
	if err != nil {
		t.Fatalf("new worker pool: %v", err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatalf("start worker pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop(context.Background()) })

	var attempts int32
	done := make(chan struct{}, 1)
	evt := domain.ChatEvent{EventID: "evt-2", AgentID: "a1", SessionID: "s1"}
	submitErr := pool.SubmitEvent(context.Background(), evt, func(context.Context) error {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			return errors.New("transient failure")
		}
		select {
		case done <- struct{}{}:
		default:
		}
		return nil
	})
	if submitErr != nil {
		t.Fatalf("submit event: %v", submitErr)
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting worker success")
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("unexpected attempts: got %d want 3", got)
	}
	if len(dlq.Records()) != 0 {
		t.Fatalf("did not expect dlq records on eventual success: %+v", dlq.Records())
	}
}
