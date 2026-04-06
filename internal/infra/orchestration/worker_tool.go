package orchestration

import (
	"context"
	"errors"

	domain "go-sse-skeleton/internal/domain/orchestration"
	port "go-sse-skeleton/internal/port/orchestration"
)

type ToolWorker struct {
	executor port.ToolExecutor
}

func NewToolWorker(executor port.ToolExecutor) (*ToolWorker, error) {
	if executor == nil {
		return nil, errors.New("nil tool executor")
	}
	return &ToolWorker{executor: executor}, nil
}

func (w *ToolWorker) ExecuteStep(ctx context.Context, _ domain.GraphState, decision domain.SupervisorDecision) (domain.WorkerOutput, error) {
	if decision.ToolName == "" {
		return domain.WorkerOutput{Content: decision.Action}, nil
	}
	content, err := w.executor.Execute(ctx, decision.ToolName, decision.ToolInput)
	if err != nil {
		return domain.WorkerOutput{}, err
	}
	return domain.WorkerOutput{Content: content}, nil
}
