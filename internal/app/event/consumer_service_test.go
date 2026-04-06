package event

import (
	"context"
	"sync"
	"testing"
	"time"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/event"
	portevent "go-sse-skeleton/internal/port/event"
)

type fakeSubscriber struct{}

func (fakeSubscriber) SubscribeChatEvent(context.Context, portevent.ChatEventHandler) error { return nil }

type fakeRunner struct {
	mu    sync.Mutex
	calls int
}

func (r *fakeRunner) RunByEvent(context.Context, domain.ChatEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return nil
}

type inlineDispatcher struct{}

func (inlineDispatcher) Start(context.Context) error { return nil }

func (inlineDispatcher) Stop(context.Context) error { return nil }

func (inlineDispatcher) Submit(ctx context.Context, job func(context.Context) error) error { return job(ctx) }

func (inlineDispatcher) SubmitEvent(ctx context.Context, _ domain.ChatEvent, job func(context.Context) error) error {
	return job(ctx)
}

type dedupStore struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func newDedupStore() *dedupStore {
	return &dedupStore{seen: map[string]struct{}{}}
}

func (s *dedupStore) MarkIfAbsent(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seen[key]; ok {
		return false, nil
	}
	s.seen[key] = struct{}{}
	return true, nil
}

type appObserver struct {
	mu       sync.Mutex
	consumed int
	failed   int
}

func (o *appObserver) RecordPublished(string) {}

func (o *appObserver) RecordConsumed(string, time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.consumed++
}

func (o *appObserver) RecordFailed(string, string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failed++
}

func (o *appObserver) RecordQueueLen(string, int) {}

type nilMessageStore struct{}

func (nilMessageStore) Create(context.Context, *chatdomain.Message) (string, error) { return "", nil }

func (nilMessageStore) GetByID(context.Context, string) (*chatdomain.Message, error) { return nil, nil }

func (nilMessageStore) ListBySession(context.Context, string) ([]*chatdomain.Message, error) { return nil, nil }

func (nilMessageStore) ListRecentBySession(context.Context, string, int) ([]*chatdomain.Message, error) {
	return nil, nil
}

func (nilMessageStore) Update(context.Context, *chatdomain.Message) error { return nil }

func (nilMessageStore) Delete(context.Context, string) error { return nil }

type noopQueue struct{}

func (noopQueue) Publish(context.Context, string, any) error { return nil }

type noopSSE struct{}

func (noopSSE) NotifyDone(context.Context, string) error { return nil }

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

func TestConsumerHandleEventIDDedup(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	obs := &appObserver{}
	svc, err := NewConsumerService(
		fakeSubscriber{},
		inlineDispatcher{},
		runner,
		nilMessageStore{},
		noopQueue{},
		noopSSE{},
		noopLLM{},
		WithDedupStore(newDedupStore()),
		WithObserver(obs),
	)
	if err != nil {
		t.Fatalf("new consumer service: %v", err)
	}

	evt := domain.ChatEvent{EventID: "evt-1", AgentID: "a1", SessionID: "s1"}
	if err = svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("first handle: %v", err)
	}
	if err = svc.Handle(context.Background(), evt); err != nil {
		t.Fatalf("second handle: %v", err)
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.calls != 1 {
		t.Fatalf("expected one runner call after dedup, got %d", runner.calls)
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if obs.consumed != 1 || obs.failed != 0 {
		t.Fatalf("unexpected observer counters: consumed=%d failed=%d", obs.consumed, obs.failed)
	}
}
