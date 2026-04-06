package orchestration

import (
	"context"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type StaticPolicy struct{}

func NewStaticPolicy() *StaticPolicy {
	return &StaticPolicy{}
}

func (p *StaticPolicy) Decide(_ context.Context, state domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, error) {
	if !eval.Pass {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusFailed, Reason: "evaluation_failed"}, nil
	}

	if state.StepIndex+1 >= state.MaxSteps {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusDone, Reason: "step_limit"}, nil
	}

	// Keep core path deterministic: once evaluation is good enough, finish early as successful.
	if eval.Score >= 0.90 {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusDone, Reason: "goal_reached"}, nil
	}

	return domain.PolicyVerdict{Continue: true}, nil
}
