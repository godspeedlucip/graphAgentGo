package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domain "go-sse-skeleton/internal/domain/orchestration"
	"go-sse-skeleton/internal/port/llm"
)

type LLMSupervisor struct {
	client llm.Client
}

func NewLLMSupervisor(client llm.Client) (*LLMSupervisor, error) {
	if client == nil {
		return nil, errors.New("nil llm client")
	}
	return &LLMSupervisor{client: client}, nil
}

func (s *LLMSupervisor) PlanNext(ctx context.Context, state domain.GraphState) (domain.SupervisorDecision, error) {
	userInput, _ := state.Metadata["userInput"].(string)
	systemPrompt, _ := state.Metadata["systemPrompt"].(string)
	prompt := fmt.Sprintf("system=%s\nstep=%d\ninput=%s\nReturn action text, or TOOL:<name>|<input>", systemPrompt, state.StepIndex, userInput)
	resp, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return domain.SupervisorDecision{}, err
	}

	text := strings.TrimSpace(resp)
	if text == "" {
		// TODO: align fallback decision strategy with Java SupervisorNode when model response is empty.
		return domain.SupervisorDecision{Action: ""}, nil
	}

	if strings.HasPrefix(strings.ToUpper(text), "TOOL:") {
		raw := strings.TrimSpace(text[5:])
		parts := strings.SplitN(raw, "|", 2)
		if len(parts) == 1 {
			return domain.SupervisorDecision{ToolName: strings.TrimSpace(parts[0]), ToolInput: userInput}, nil
		}
		return domain.SupervisorDecision{ToolName: strings.TrimSpace(parts[0]), ToolInput: strings.TrimSpace(parts[1])}, nil
	}

	return domain.SupervisorDecision{Action: text}, nil
}
