package orchestration

import (
	"context"
	"sync"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type InMemoryTraceRepository struct {
	mu    sync.Mutex
	trace map[string][]domain.StepTrace
}

func NewInMemoryTraceRepository() *InMemoryTraceRepository {
	return &InMemoryTraceRepository{trace: make(map[string][]domain.StepTrace)}
}

func (r *InMemoryTraceRepository) AppendStep(_ context.Context, runID string, step domain.StepTrace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trace[runID] = append(r.trace[runID], step)
	return nil
}

type NoopEventPublisher struct{}

func NewNoopEventPublisher() *NoopEventPublisher { return &NoopEventPublisher{} }

func (p *NoopEventPublisher) PublishGraphEvent(_ context.Context, _ string, _ any) error { return nil }

type NoopStreamNotifier struct{}

func NewNoopStreamNotifier() *NoopStreamNotifier { return &NoopStreamNotifier{} }

func (n *NoopStreamNotifier) NotifyStep(_ context.Context, _ string, _ domain.StepTrace) error { return nil }
func (n *NoopStreamNotifier) NotifyFinal(_ context.Context, _ string, _ domain.GraphResult) error { return nil }
