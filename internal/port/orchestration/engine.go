package orchestration

import (
	"context"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type GraphEngine interface {
	Execute(ctx context.Context, in domain.GraphInput) (domain.GraphResult, error)
}

type Supervisor interface {
	PlanNext(ctx context.Context, state domain.GraphState) (domain.SupervisorDecision, error)
}

type Worker interface {
	ExecuteStep(ctx context.Context, state domain.GraphState, decision domain.SupervisorDecision) (domain.WorkerOutput, error)
}

type Evaluator interface {
	Evaluate(ctx context.Context, state domain.GraphState, out domain.WorkerOutput) (domain.Evaluation, error)
}

type Policy interface {
	Decide(ctx context.Context, state domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, error)
}
