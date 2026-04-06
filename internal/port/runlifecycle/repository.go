package runlifecycle

import (
	"context"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type RunRepository interface {
	Create(ctx context.Context, rec domain.RunRecord) error
	Get(ctx context.Context, runID string) (*domain.RunRecord, error)
	UpdateStatus(ctx context.Context, runID string, status domain.Status, metadata map[string]any) error
	AppendOutput(ctx context.Context, runID string, delta string) error
}
