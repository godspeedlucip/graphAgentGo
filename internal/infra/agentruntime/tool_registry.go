package agentruntime

import (
	"context"

	domain "go-sse-skeleton/internal/domain/agentruntime"
)

type ToolRegistry struct {
	fixed    []domain.ToolDef
	optional []domain.ToolDef
}

func NewToolRegistry(fixed []domain.ToolDef, optional []domain.ToolDef) *ToolRegistry {
	return &ToolRegistry{
		fixed:    append([]domain.ToolDef(nil), fixed...),
		optional: append([]domain.ToolDef(nil), optional...),
	}
}

func (r *ToolRegistry) FixedTools(_ context.Context) ([]domain.ToolDef, error) {
	return append([]domain.ToolDef(nil), r.fixed...), nil
}

func (r *ToolRegistry) OptionalTools(_ context.Context) ([]domain.ToolDef, error) {
	return append([]domain.ToolDef(nil), r.optional...), nil
}

