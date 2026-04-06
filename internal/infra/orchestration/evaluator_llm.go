package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domain "go-sse-skeleton/internal/domain/orchestration"
	"go-sse-skeleton/internal/port/llm"
)

type LLMEvaluator struct {
	client llm.Client
}

func NewLLMEvaluator(client llm.Client) (*LLMEvaluator, error) {
	if client == nil {
		return nil, errors.New("nil llm client")
	}
	return &LLMEvaluator{client: client}, nil
}

func (e *LLMEvaluator) Evaluate(ctx context.Context, state domain.GraphState, out domain.WorkerOutput) (domain.Evaluation, error) {
	if strings.TrimSpace(out.Content) == "" {
		return domain.Evaluation{Pass: false, Score: 0, Feedback: "empty_output"}, nil
	}

	prompt := fmt.Sprintf("step=%d\ncontent=%s\nReturn PASS or FAIL", state.StepIndex, out.Content)
	resp, err := e.client.Generate(ctx, prompt)
	if err != nil {
		return domain.Evaluation{}, err
	}
	text := strings.ToUpper(strings.TrimSpace(resp))
	if strings.Contains(text, "FAIL") {
		return domain.Evaluation{Pass: false, Score: 0.2, Feedback: strings.TrimSpace(resp)}, nil
	}

	// TODO: align scoring formula with Java StepEvaluator confidence computation.
	return domain.Evaluation{Pass: true, Score: 0.95, Feedback: strings.TrimSpace(resp)}, nil
}
