package sse

import (
	"context"
	"time"
)

type Telemetry interface {
	RecordSend(ctx context.Context, eventType string, errClass string, degraded bool, latency time.Duration)
	RecordAssistantPlaceholderCreate(ctx context.Context, sessionID string, errClass string, ok bool, latency time.Duration)
}

type ServiceOption func(*Service)

func WithTelemetry(t Telemetry) ServiceOption {
	return func(s *Service) {
		if t != nil {
			s.telemetry = t
		}
	}
}

type NoopTelemetry struct{}

func NewNoopTelemetry() *NoopTelemetry { return &NoopTelemetry{} }

func (n *NoopTelemetry) RecordSend(_ context.Context, _ string, _ string, _ bool, _ time.Duration) {}

func (n *NoopTelemetry) RecordAssistantPlaceholderCreate(_ context.Context, _ string, _ string, _ bool, _ time.Duration) {
}
