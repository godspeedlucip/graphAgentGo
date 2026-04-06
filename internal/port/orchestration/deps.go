package orchestration

import (
	"context"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, input string) (string, error)
}

type TraceRepository interface {
	AppendStep(ctx context.Context, runID string, step domain.StepTrace) error
}

type EventPublisher interface {
	PublishGraphEvent(ctx context.Context, topic string, payload any) error
}

type StreamNotifier interface {
	NotifyStep(ctx context.Context, sessionID string, step domain.StepTrace) error
	NotifyFinal(ctx context.Context, sessionID string, result domain.GraphResult) error
}
