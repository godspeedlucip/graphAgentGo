package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type stubSupervisor struct {
	fn func(context.Context, domain.GraphState) (domain.SupervisorDecision, error)
}

func (s stubSupervisor) PlanNext(ctx context.Context, st domain.GraphState) (domain.SupervisorDecision, error) {
	return s.fn(ctx, st)
}

type stubWorker struct {
	fn func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error)
}

func (w stubWorker) ExecuteStep(ctx context.Context, st domain.GraphState, d domain.SupervisorDecision) (domain.WorkerOutput, error) {
	return w.fn(ctx, st, d)
}

type stubEvaluator struct {
	fn func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error)
}

func (e stubEvaluator) Evaluate(ctx context.Context, st domain.GraphState, out domain.WorkerOutput) (domain.Evaluation, error) {
	return e.fn(ctx, st, out)
}

type stubPolicy struct {
	fn func(context.Context, domain.GraphState, domain.Evaluation) (domain.PolicyVerdict, error)
}

func (p stubPolicy) Decide(ctx context.Context, st domain.GraphState, ev domain.Evaluation) (domain.PolicyVerdict, error) {
	return p.fn(ctx, st, ev)
}

func TestGraphEngineExecuteConverges(t *testing.T) {
	t.Parallel()

	engine, err := NewGraphEngine(
		stubSupervisor{fn: func(context.Context, domain.GraphState) (domain.SupervisorDecision, error) {
			return domain.SupervisorDecision{Action: "answer", DecisionType: "answer"}, nil
		}},
		stubWorker{fn: func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error) {
			return domain.WorkerOutput{Content: "ok"}, nil
		}},
		stubEvaluator{fn: func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error) {
			return domain.Evaluation{Pass: true, Score: 0.95, Decision: "pass"}, nil
		}},
		NewStaticPolicy(),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	result, execErr := engine.Execute(context.Background(), domain.GraphInput{
		RunID: "run-1", AgentID: "agent-1", SessionID: "session-1", UserInput: "hello", MaxSteps: 3,
	})
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if result.Status != domain.FinalStatusDone {
		t.Fatalf("expected done, got %s", result.Status)
	}
	if result.StepsUsed != 1 || result.Reply != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGraphEngineExecuteWorkerError(t *testing.T) {
	t.Parallel()

	engine, _ := NewGraphEngine(
		stubSupervisor{fn: func(context.Context, domain.GraphState) (domain.SupervisorDecision, error) {
			return domain.SupervisorDecision{ToolName: "db", ToolInput: "x", DecisionType: "tool"}, nil
		}},
		stubWorker{fn: func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error) {
			return domain.WorkerOutput{}, errors.New("tool failed")
		}},
		stubEvaluator{fn: func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error) {
			return domain.Evaluation{Pass: true, Score: 1, Decision: "pass"}, nil
		}},
		NewStaticPolicy(),
	)

	result, err := engine.Execute(context.Background(), domain.GraphInput{
		RunID: "run-2", AgentID: "agent-1", SessionID: "session-1", UserInput: "do", MaxSteps: 3,
	})
	if err == nil || err.Error() != "tool failed" {
		t.Fatalf("expected tool failed error, got %v", err)
	}
	if result.Status != domain.FinalStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
}

func TestGraphEngineExecuteEvaluatorError(t *testing.T) {
	t.Parallel()

	engine, _ := NewGraphEngine(
		stubSupervisor{fn: func(context.Context, domain.GraphState) (domain.SupervisorDecision, error) {
			return domain.SupervisorDecision{Action: "continue", DecisionType: "answer"}, nil
		}},
		stubWorker{fn: func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error) {
			return domain.WorkerOutput{Content: "mid"}, nil
		}},
		stubEvaluator{fn: func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error) {
			return domain.Evaluation{}, errors.New("eval failed")
		}},
		NewStaticPolicy(),
	)

	result, err := engine.Execute(context.Background(), domain.GraphInput{
		RunID: "run-3", AgentID: "agent-1", SessionID: "session-1", UserInput: "do", MaxSteps: 3,
	})
	if err == nil || err.Error() != "eval failed" {
		t.Fatalf("expected eval failed error, got %v", err)
	}
	if result.Status != domain.FinalStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
}

func TestGraphEngineExecuteTimeout(t *testing.T) {
	t.Parallel()

	engine, _ := NewGraphEngine(
		stubSupervisor{fn: func(ctx context.Context, _ domain.GraphState) (domain.SupervisorDecision, error) {
			<-ctx.Done()
			return domain.SupervisorDecision{}, ctx.Err()
		}},
		stubWorker{fn: func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error) {
			return domain.WorkerOutput{Content: "never"}, nil
		}},
		stubEvaluator{fn: func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error) {
			return domain.Evaluation{Pass: true, Score: 1, Decision: "pass"}, nil
		}},
		NewStaticPolicy(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result, err := engine.Execute(ctx, domain.GraphInput{
		RunID: "run-4", AgentID: "agent-1", SessionID: "session-1", UserInput: "slow", MaxSteps: 3,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.Status != domain.FinalStatusTimeout {
		t.Fatalf("expected timeout status, got %s", result.Status)
	}
}

func TestGraphEngineExecuteStepLimit(t *testing.T) {
	t.Parallel()

	engine, _ := NewGraphEngine(
		stubSupervisor{fn: func(context.Context, domain.GraphState) (domain.SupervisorDecision, error) {
			return domain.SupervisorDecision{Action: "next", DecisionType: "answer"}, nil
		}},
		stubWorker{fn: func(context.Context, domain.GraphState, domain.SupervisorDecision) (domain.WorkerOutput, error) {
			return domain.WorkerOutput{Content: "mid"}, nil
		}},
		stubEvaluator{fn: func(context.Context, domain.GraphState, domain.WorkerOutput) (domain.Evaluation, error) {
			return domain.Evaluation{Pass: true, Score: 0.20, Decision: "pass"}, nil
		}},
		stubPolicy{fn: func(context.Context, domain.GraphState, domain.Evaluation) (domain.PolicyVerdict, error) {
			return domain.PolicyVerdict{Continue: true}, nil
		}},
	)

	result, err := engine.Execute(context.Background(), domain.GraphInput{
		RunID: "run-5", AgentID: "agent-1", SessionID: "session-1", UserInput: "loop", MaxSteps: 2,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Reason != domain.ErrStepLimitExceeded.Error() || result.StepsUsed != 2 {
		t.Fatalf("unexpected step-limit result: %+v", result)
	}
}

