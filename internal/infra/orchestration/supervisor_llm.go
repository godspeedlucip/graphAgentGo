package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domain "go-sse-skeleton/internal/domain/orchestration"
	"go-sse-skeleton/internal/port/llm"
)

type LLMSupervisor struct {
	client llm.Client
}

const supervisorProtocolV1 = "orchestration.supervisor.v1"

type supervisorEnvelope struct {
	Protocol string             `json:"protocol"`
	Decision supervisorDecision `json:"decision"`
}

type supervisorDecision struct {
	Type       string  `json:"type"`
	Action     string  `json:"action,omitempty"`
	ToolName   string  `json:"tool_name,omitempty"`
	ToolInput  string  `json:"tool_input,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
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
	prompt := fmt.Sprintf(
		"system=%s\nstep=%d\ninput=%s\n"+
			"Return STRICT JSON object with schema:\n"+
			"{\"protocol\":\"%s\",\"decision\":{\"type\":\"answer|tool|finish\",\"action\":\"string?\",\"tool_name\":\"string?\",\"tool_input\":\"string?\",\"reason\":\"string?\",\"confidence\":0.0}}\n"+
			"Rules:\n"+
			"1) Use type=tool only when tool execution is needed.\n"+
			"2) type=tool requires tool_name.\n"+
			"3) type=answer requires action.\n"+
			"4) Do not output markdown or extra fields.",
		systemPrompt, state.StepIndex, userInput, supervisorProtocolV1,
	)
	resp, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return domain.SupervisorDecision{}, err
	}

	text := strings.TrimSpace(resp)
	if text == "" {
		return domain.SupervisorDecision{}, errors.New("empty supervisor response")
	}

	parsed, parseErr := parseSupervisorJSON(text, userInput)
	if parseErr == nil {
		return parsed, nil
	}
	if strings.HasPrefix(text, "{") {
		return domain.SupervisorDecision{}, parseErr
	}

	legacy, ok := parseLegacyToolDecision(text, userInput)
	if ok {
		return legacy, nil
	}

	return domain.SupervisorDecision{}, parseErr
}

func parseSupervisorJSON(raw string, defaultToolInput string) (domain.SupervisorDecision, error) {
	var env supervisorEnvelope
	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return domain.SupervisorDecision{}, fmt.Errorf("decode supervisor json: %w", err)
	}
	if strings.TrimSpace(env.Protocol) != "" && env.Protocol != supervisorProtocolV1 {
		return domain.SupervisorDecision{}, fmt.Errorf("unsupported supervisor protocol: %s", env.Protocol)
	}
	t := strings.ToLower(strings.TrimSpace(env.Decision.Type))
	if t == "" {
		return domain.SupervisorDecision{}, errors.New("missing decision.type")
	}
	switch t {
	case "answer":
		action := strings.TrimSpace(env.Decision.Action)
		if action == "" {
			return domain.SupervisorDecision{}, errors.New("answer decision requires action")
		}
		return domain.SupervisorDecision{
			Action:       action,
			DecisionType: "answer",
			Reason:       strings.TrimSpace(env.Decision.Reason),
			Confidence:   normalizeConfidence(env.Decision.Confidence),
			Metadata:     map[string]any{"protocol": supervisorProtocolV1},
		}, nil
	case "tool":
		toolName := strings.TrimSpace(env.Decision.ToolName)
		if toolName == "" {
			return domain.SupervisorDecision{}, errors.New("tool decision requires tool_name")
		}
		toolInput := strings.TrimSpace(env.Decision.ToolInput)
		if toolInput == "" {
			toolInput = defaultToolInput
		}
		return domain.SupervisorDecision{
			ToolName:     toolName,
			ToolInput:    toolInput,
			DecisionType: "tool",
			Reason:       strings.TrimSpace(env.Decision.Reason),
			Confidence:   normalizeConfidence(env.Decision.Confidence),
			Metadata:     map[string]any{"protocol": supervisorProtocolV1},
		}, nil
	case "finish":
		// Keep minimal compatibility with current serial engine: treat finish as final answer text.
		action := strings.TrimSpace(env.Decision.Action)
		if action == "" {
			action = strings.TrimSpace(env.Decision.Reason)
		}
		if action == "" {
			action = "done"
		}
		return domain.SupervisorDecision{
			Action:       action,
			DecisionType: "finish",
			Reason:       strings.TrimSpace(env.Decision.Reason),
			Confidence:   normalizeConfidence(env.Decision.Confidence),
			Metadata:     map[string]any{"protocol": supervisorProtocolV1},
		}, nil
	default:
		return domain.SupervisorDecision{}, fmt.Errorf("unsupported decision.type: %s", t)
	}
}

func parseLegacyToolDecision(text string, userInput string) (domain.SupervisorDecision, bool) {
	if strings.HasPrefix(strings.ToUpper(text), "TOOL:") {
		raw := strings.TrimSpace(text[5:])
		parts := strings.SplitN(raw, "|", 2)
		if len(parts) == 1 {
			return domain.SupervisorDecision{
				ToolName:     strings.TrimSpace(parts[0]),
				ToolInput:    userInput,
				DecisionType: "tool",
				Metadata:     map[string]any{"protocol": "legacy-text"},
			}, true
		}
		return domain.SupervisorDecision{
			ToolName:     strings.TrimSpace(parts[0]),
			ToolInput:    strings.TrimSpace(parts[1]),
			DecisionType: "tool",
			Metadata:     map[string]any{"protocol": "legacy-text"},
		}, true
	}
	if text != "" {
		return domain.SupervisorDecision{
			Action:       text,
			DecisionType: "answer",
			Metadata:     map[string]any{"protocol": "legacy-text"},
		}, true
	}
	return domain.SupervisorDecision{}, false
}

func normalizeConfidence(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
