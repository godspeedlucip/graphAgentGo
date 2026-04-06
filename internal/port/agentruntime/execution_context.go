package agentruntime

import (
	"context"

	chatdomain "go-sse-skeleton/internal/domain/chat"
)

type ExecutionHooks struct {
	RecentMessages    []*chatdomain.Message
	AssistantMessageID string
	AppendDelta       func(ctx context.Context, delta string) error
	CollectDelta      func(delta string)
}

type executionHooksKey struct{}

func WithExecutionHooks(ctx context.Context, hooks ExecutionHooks) context.Context {
	return context.WithValue(ctx, executionHooksKey{}, hooks)
}

func ExecutionHooksFromContext(ctx context.Context) (ExecutionHooks, bool) {
	if ctx == nil {
		return ExecutionHooks{}, false
	}
	v := ctx.Value(executionHooksKey{})
	if v == nil {
		return ExecutionHooks{}, false
	}
	hooks, ok := v.(ExecutionHooks)
	return hooks, ok
}
