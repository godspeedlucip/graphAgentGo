package agentruntime

import (
	"context"

	domain "go-sse-skeleton/internal/domain/agentruntime"
)

type AgentConfigRepository interface {
	GetAgentConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error)
	ListKnowledgeBases(ctx context.Context, kbIDs []string) ([]domain.KnowledgeBase, error)
}

type ToolRegistry interface {
	FixedTools(ctx context.Context) ([]domain.ToolDef, error)
	OptionalTools(ctx context.Context) ([]domain.ToolDef, error)
}