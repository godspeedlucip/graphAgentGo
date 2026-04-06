package runlifecycle

import (
	"context"
	"sync"
)

type RuntimeRegistry interface {
	Register(runID string, cancel context.CancelFunc)
	Cancel(runID string) bool
	Unregister(runID string)
}

type runtimeRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewRuntimeRegistry() RuntimeRegistry {
	return &runtimeRegistry{cancels: map[string]context.CancelFunc{}}
}

func (r *runtimeRegistry) Register(runID string, cancel context.CancelFunc) {
	if runID == "" || cancel == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[runID] = cancel
}

func (r *runtimeRegistry) Cancel(runID string) bool {
	if runID == "" {
		return false
	}
	r.mu.Lock()
	cancel, ok := r.cancels[runID]
	r.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

func (r *runtimeRegistry) Unregister(runID string) {
	if runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, runID)
}
