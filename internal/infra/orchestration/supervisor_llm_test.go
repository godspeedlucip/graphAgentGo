package orchestration

import (
	"context"
	"testing"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type fakeLLMClient struct {
	resp string
	err  error
}

func (c fakeLLMClient) Generate(context.Context, string) (string, error) {
	return c.resp, c.err
}

func TestLLMSupervisorParseStructuredToolDecision(t *testing.T) {
	t.Parallel()

	s, err := NewLLMSupervisor(fakeLLMClient{
		resp: `{"protocol":"orchestration.supervisor.v1","decision":{"type":"tool","tool_name":"weather","tool_input":"shanghai","confidence":0.92}}`,
	})
	if err != nil {
		t.Fatalf("new supervisor: %v", err)
	}
	decision, planErr := s.PlanNext(context.Background(), domain.GraphState{
		StepIndex: 1,
		Metadata:  map[string]any{"userInput": "天气"},
	})
	if planErr != nil {
		t.Fatalf("plan next: %v", planErr)
	}
	if decision.ToolName != "weather" || decision.ToolInput != "shanghai" || decision.DecisionType != "tool" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestLLMSupervisorStrictJSONValidation(t *testing.T) {
	t.Parallel()

	s, err := NewLLMSupervisor(fakeLLMClient{
		resp: `{"protocol":"orchestration.supervisor.v1","decision":{"type":"answer","action":"ok","unknown":"x"}}`,
	})
	if err != nil {
		t.Fatalf("new supervisor: %v", err)
	}
	_, planErr := s.PlanNext(context.Background(), domain.GraphState{
		StepIndex: 1,
		Metadata:  map[string]any{"userInput": "hi"},
	})
	if planErr == nil {
		t.Fatal("expected strict json validation error")
	}
}

