package agentruntime

import (
	"context"

	domain "go-sse-skeleton/internal/domain/agentruntime"
)

type ChatClient interface{}

type ChatClientRegistry interface {
	GetByModel(ctx context.Context, model string) (ChatClient, error)
}

type Memory interface{}

type MemoryBuilder interface {
	Build(ctx context.Context, sessionID string, maxMessages int) (Memory, error)
}

type GraphRuntime interface {
	Execute(ctx context.Context) error
}

type GraphBuilder interface {
	Build(ctx context.Context, input GraphBuildInput) (GraphRuntime, error)
}

type GraphBuildInput struct {
	AgentID      string
	SessionID    string
	SystemPrompt string
	Tools        []string
	Knowledge    []domain.KnowledgeBase
	ModelClient  ChatClient
	Memory       Memory
}
