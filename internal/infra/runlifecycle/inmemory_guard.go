package runlifecycle

import (
	"context"
	"sync"
	"time"
)

type InMemoryGuard struct {
	mu    sync.Mutex
	locks map[string]time.Time
}

func NewInMemoryGuard() *InMemoryGuard {
	return &InMemoryGuard{locks: make(map[string]time.Time)}
}

func (g *InMemoryGuard) Acquire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if exp, ok := g.locks[key]; ok && exp.After(now) {
		return false, nil
	}
	g.locks[key] = now.Add(ttl)
	return true, nil
}

func (g *InMemoryGuard) Release(_ context.Context, key string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.locks, key)
	return nil
}
