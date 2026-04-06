package orchestration

import (
	"context"
	"testing"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

func TestLLMEvaluatorParseStructuredResult(t *testing.T) {
	t.Parallel()

	e, err := NewLLMEvaluator(fakeLLMClient{
		resp: `{"protocol":"orchestration.evaluator.v1","result":{"pass":true,"score":0.88,"feedback":"good","decision":"pass"}}`,
	})
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	out, evalErr := e.Evaluate(context.Background(), domain.GraphState{StepIndex: 0}, domain.WorkerOutput{Content: "ok"})
	if evalErr != nil {
		t.Fatalf("evaluate: %v", evalErr)
	}
	if !out.Pass || out.Decision != "pass" || out.Score != 0.88 {
		t.Fatalf("unexpected evaluation: %+v", out)
	}
}

func TestLLMEvaluatorMetadataPolicyViolation(t *testing.T) {
	t.Parallel()

	e, err := NewLLMEvaluator(fakeLLMClient{resp: "PASS"})
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	out, evalErr := e.Evaluate(context.Background(), domain.GraphState{}, domain.WorkerOutput{
		Content:  "failed",
		Metadata: map[string]any{"tool_failed": true, "error": "policy violation: forbidden tool"},
	})
	if evalErr != nil {
		t.Fatalf("evaluate: %v", evalErr)
	}
	if out.Decision != "retry" || !out.PolicyViolation || !out.Retryable {
		t.Fatalf("unexpected policy violation mapping: %+v", out)
	}
}

func TestLLMEvaluatorStrictJSONValidation(t *testing.T) {
	t.Parallel()

	e, err := NewLLMEvaluator(fakeLLMClient{
		resp: `{"protocol":"orchestration.evaluator.v1","result":{"pass":false,"score":0.1,"feedback":"x","decision":"fail","unknown":"x"}}`,
	})
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	_, evalErr := e.Evaluate(context.Background(), domain.GraphState{}, domain.WorkerOutput{Content: "bad"})
	if evalErr == nil {
		t.Fatal("expected strict json validation error")
	}
}

