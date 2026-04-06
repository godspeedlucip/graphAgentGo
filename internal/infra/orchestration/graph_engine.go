package orchestration

import (
	"context"
	"errors"

	domain "go-sse-skeleton/internal/domain/orchestration"
	port "go-sse-skeleton/internal/port/orchestration"
)

type GraphEngine struct {
	supervisor port.Supervisor
	worker     port.Worker
	evaluator  port.Evaluator
	policy     port.Policy
}

func NewGraphEngine(supervisor port.Supervisor, worker port.Worker, evaluator port.Evaluator, policy port.Policy) (*GraphEngine, error) {
	if supervisor == nil || worker == nil || evaluator == nil || policy == nil {
		return nil, errors.New("nil graph engine dependency")
	}
	return &GraphEngine{supervisor: supervisor, worker: worker, evaluator: evaluator, policy: policy}, nil
}

func (e *GraphEngine) Execute(ctx context.Context, in domain.GraphInput) (domain.GraphResult, error) {
	if in.RunID == "" || in.AgentID == "" || in.SessionID == "" {
		return domain.GraphResult{}, domain.ErrInvalidInput
	}

	state := domain.GraphState{
		RunID:     in.RunID,
		AgentID:   in.AgentID,
		SessionID: in.SessionID,
		StepIndex: 0,
		MaxSteps:  domain.NormalizeMaxSteps(in.MaxSteps),
		Metadata:  cloneMeta(in.Metadata),
	}
	state.Metadata["userInput"] = in.UserInput
	state.Metadata["systemPrompt"] = in.SystemPrompt

	trace := make([]domain.StepTrace, 0, state.MaxSteps)
	for !domain.ShouldStop(state) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.GraphResult{
				Status:    mapContextStatus(ctxErr),
				Reason:    ctxErr.Error(),
				Reply:     state.FinalReply,
				StepsUsed: len(trace),
				Trace:     trace,
			}, ctxErr
		}

		decision, err := e.supervisor.PlanNext(ctx, state)
		if err != nil {
			return failResult(state, trace, err), err
		}

		out, err := e.worker.ExecuteStep(ctx, state, decision)
		if err != nil {
			return failResult(state, trace, err), err
		}

		eval, err := e.evaluator.Evaluate(ctx, state, out)
		if err != nil {
			return failResult(state, trace, err), err
		}

		verdict, err := e.policy.Decide(ctx, state, eval)
		if err != nil {
			return failResult(state, trace, err), err
		}

		step := domain.StepTrace{
			StepIndex:  state.StepIndex,
			Decision:   decision,
			Worker:     out,
			Evaluation: eval,
			Verdict:    verdict,
		}
		trace = append(trace, step)
		state.StepIndex++
		if out.Content != "" {
			state.FinalReply = out.Content
		}

		if !verdict.Continue {
			final := verdict.Final
			if final == "" {
				final = domain.FinalStatusDone
			}
			state.Final = final
			state.Reason = verdict.Reason
			state.Stopped = true
			if state.Reason == "" {
				state.Reason = "policy_stop"
			}
			return domain.GraphResult{
				Status:    state.Final,
				Reason:    state.Reason,
				Reply:     state.FinalReply,
				StepsUsed: len(trace),
				Trace:     trace,
			}, nil
		}
	}

	// This branch is reached when step budget is exhausted without explicit policy stop.
	// Keep a deterministic fallback result and return no hard error.
	return domain.GraphResult{
		Status:    domain.FinalStatusDone,
		Reason:    domain.ErrStepLimitExceeded.Error(),
		Reply:     state.FinalReply,
		StepsUsed: len(trace),
		Trace:     trace,
	}, nil
}

func failResult(state domain.GraphState, trace []domain.StepTrace, err error) domain.GraphResult {
	return domain.GraphResult{
		Status:    domain.FinalStatusFailed,
		Reason:    err.Error(),
		Reply:     state.FinalReply,
		StepsUsed: len(trace),
		Trace:     trace,
	}
}

func mapContextStatus(err error) domain.FinalStatus {
	if errors.Is(err, context.Canceled) {
		return domain.FinalStatusCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.FinalStatusTimeout
	}
	return domain.FinalStatusFailed
}

func cloneMeta(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

var _ port.GraphEngine = (*GraphEngine)(nil)
