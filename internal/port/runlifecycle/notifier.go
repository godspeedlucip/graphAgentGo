package runlifecycle

import (
	"context"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type EventPublisher interface {
	PublishLifecycle(ctx context.Context, evt domain.LifecycleEvent) error
}

type SSENotifier interface {
	NotifyStarted(ctx context.Context, runID string, sessionID string) error
	NotifyDelta(ctx context.Context, runID string, sessionID string, delta string) error
	NotifyDone(ctx context.Context, runID string, sessionID string) error
	NotifyFailed(ctx context.Context, runID string, sessionID string, reason string) error
}
