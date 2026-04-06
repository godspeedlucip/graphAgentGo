package agentruntime

import (
	"context"

	port "go-sse-skeleton/internal/port/agentruntime"
)

type RuntimeAssembleInput struct {
	AgentID   string
	SessionID string
	Graph     port.GraphRuntime
}

type RuntimeAssembler interface {
	Assemble(ctx context.Context, in RuntimeAssembleInput) (port.Runtime, error)
}