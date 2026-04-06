package event

import (
	"context"
	"sync"
	"time"
)

type InMemoryDedupStore struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func NewInMemoryDedupStore() *InMemoryDedupStore {
	return &InMemoryDedupStore{items: map[string]time.Time{}}
}

func (s *InMemoryDedupStore) MarkIfAbsent(_ context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expireAt := now.Add(ttl)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Lazy cleanup for expired keys.
	for k, exp := range s.items {
		if now.After(exp) {
			delete(s.items, k)
		}
	}

	if exp, ok := s.items[key]; ok && now.Before(exp) {
		return false, nil
	}
	s.items[key] = expireAt
	return true, nil
}
