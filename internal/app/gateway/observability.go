package gateway

import domain "go-sse-skeleton/internal/domain/gateway"

type Observer interface {
	RecordTargetHit(target domain.Target, reason string)
	RecordFallback(from domain.Target, to domain.Target, reason string, method string, path string)
	RecordSSEDisconnect(target domain.Target, reason string)
}

type Option func(*service)

func WithObserver(observer Observer) Option {
	return func(s *service) {
		if observer != nil {
			s.observer = observer
		}
	}
}

type NoopObserver struct{}

func NewNoopObserver() *NoopObserver { return &NoopObserver{} }

func (n *NoopObserver) RecordTargetHit(domain.Target, string) {}

func (n *NoopObserver) RecordFallback(domain.Target, domain.Target, string, string, string) {}

func (n *NoopObserver) RecordSSEDisconnect(domain.Target, string) {}
