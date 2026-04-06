package runlifecycle

import (
	"context"
	"sync"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type InMemoryRunRepository struct {
	mu   sync.RWMutex
	runs map[string]domain.RunRecord
}

func NewInMemoryRunRepository() *InMemoryRunRepository {
	return &InMemoryRunRepository{runs: make(map[string]domain.RunRecord)}
}

func (r *InMemoryRunRepository) Create(_ context.Context, rec domain.RunRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[rec.RunID]; exists {
		return domain.ErrRunAlreadyExists
	}
	r.runs[rec.RunID] = rec
	return nil
}

func (r *InMemoryRunRepository) Get(_ context.Context, runID string) (*domain.RunRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.runs[runID]
	if !ok {
		return nil, nil
	}
	copied := rec
	return &copied, nil
}

func (r *InMemoryRunRepository) UpdateStatus(_ context.Context, runID string, status domain.Status, metadata map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.runs[runID]
	if !ok {
		return domain.ErrRunNotFound
	}
	rec.Status = status
	if metadata != nil {
		rec.Metadata = metadata
	}
	r.runs[runID] = rec
	return nil
}

func (r *InMemoryRunRepository) AppendOutput(_ context.Context, runID string, delta string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.runs[runID]
	if !ok {
		return domain.ErrRunNotFound
	}
	rec.Output += delta
	r.runs[runID] = rec
	return nil
}
