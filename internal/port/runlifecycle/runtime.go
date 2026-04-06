package runlifecycle

import (
	"context"
	"time"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type Runtime interface {
	Run(ctx context.Context, in RuntimeInput) (RuntimeOutput, error)
}

type RuntimeInput struct {
	RunID     string
	AgentID   string
	SessionID string
	UserInput string
	Metadata  map[string]any
	// AppendOutput enables incremental output persistence/streaming.
	AppendOutput func(ctx context.Context, delta string) error
}

type RuntimeOutput struct {
	Text     string
	Metadata map[string]any
}

type RuntimeProvider interface {
	GetRuntime(ctx context.Context, agentID string, sessionID string) (Runtime, error)
}

type IdempotencyGuard interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

type Clock interface {
	Now() time.Time
}
