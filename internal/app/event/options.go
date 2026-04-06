package event

import (
	"context"
	"time"

	port "go-sse-skeleton/internal/port/event"
)

type Option func(*consumerService)

func WithDedupStore(store port.DedupStore) Option {
	return func(s *consumerService) {
		if store != nil {
			s.dedup = store
		}
	}
}

func WithObserver(observer port.Observer) Option {
	return func(s *consumerService) {
		if observer != nil {
			s.observer = observer
		}
	}
}

func WithDedupTTL(ttl time.Duration) Option {
	return func(s *consumerService) {
		if ttl > 0 {
			s.dedupTTL = ttl
		}
	}
}

type PublisherOption func(*publisherService)

func WithPublisherObserver(observer port.Observer) PublisherOption {
	return func(s *publisherService) {
		if observer != nil {
			s.observer = observer
		}
	}
}

type noopObserver struct{}

func (noopObserver) RecordPublished(string) {}

func (noopObserver) RecordConsumed(string, time.Duration) {}

func (noopObserver) RecordFailed(string, string) {}

func (noopObserver) RecordQueueLen(string, int) {}

type noopDedupStore struct{}

func (noopDedupStore) MarkIfAbsent(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}
