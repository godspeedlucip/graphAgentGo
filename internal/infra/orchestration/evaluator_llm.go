package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	domain "go-sse-skeleton/internal/domain/orchestration"
	"go-sse-skeleton/internal/port/llm"
)

type LLMEvaluator struct {
	client llm.Client
}

const evaluatorProtocolV1 = "orchestration.evaluator.v1"

type evaluatorEnvelope struct {
	Protocol string               `json:"protocol"`
	Result   evaluatorResultModel `json:"result"`
}

type evaluatorResultModel struct {
	Pass            bool    `json:"pass"`
	Score           float64 `json:"score"`
	Feedback        string  `json:"feedback"`
	Decision        string  `json:"decision"`
	Reason          string  `json:"reason,omitempty"`
	Retryable       bool    `json:"retryable,omitempty"`
	PolicyViolation bool    `json:"policy_violation,omitempty"`
}

func NewLLMEvaluator(client llm.Client) (*LLMEvaluator, error) {
	if client == nil {
		return nil, errors.New("nil llm client")
	}
	return &LLMEvaluator{client: client}, nil
}

func (e *LLMEvaluator) Evaluate(ctx context.Context, state domain.GraphState, out domain.WorkerOutput) (domain.Evaluation, error) {
	content := strings.TrimSpace(out.Content)
	if content == "" {
		return domain.Evaluation{
			Pass:      false,
			Score:     0,
			Feedback:  "empty_output",
			Decision:  "retry",
			Retryable: true,
		}, nil
	}

	if eval, ok := fromWorkerMetadata(out.Metadata); ok {
		return eval, nil
	}

	prompt := fmt.Sprintf(
		"step=%d\ncontent=%s\n"+
			"Return STRICT JSON object with schema:\n"+
			"{\"protocol\":\"%s\",\"result\":{\"pass\":true,\"score\":0.0,\"feedback\":\"string\",\"decision\":\"pass|retry|reclassify|fail\",\"reason\":\"string?\",\"retryable\":false,\"policy_violation\":false}}\n"+
			"Rules:\n"+
			"1) score must be [0,1]\n"+
			"2) pass=true only with decision=pass\n"+
			"3) policy/tool/domain issues should map to retry/reclassify/fail clearly\n"+
			"4) no markdown or extra fields",
		state.StepIndex, out.Content, evaluatorProtocolV1,
	)
	resp, err := e.client.Generate(ctx, prompt)
	if err != nil {
		return domain.Evaluation{}, err
	}
	parsed, parseErr := parseEvaluatorJSON(resp)
	if parseErr == nil {
		return parsed, nil
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "{") {
		return domain.Evaluation{}, parseErr
	}
	return fallbackEvaluator(strings.TrimSpace(resp)), nil
}

func parseEvaluatorJSON(raw string) (domain.Evaluation, error) {
	var env evaluatorEnvelope
	dec := json.NewDecoder(bytes.NewBufferString(strings.TrimSpace(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return domain.Evaluation{}, fmt.Errorf("decode evaluator json: %w", err)
	}
	if strings.TrimSpace(env.Protocol) != "" && env.Protocol != evaluatorProtocolV1 {
		return domain.Evaluation{}, fmt.Errorf("unsupported evaluator protocol: %s", env.Protocol)
	}
	decision := strings.ToLower(strings.TrimSpace(env.Result.Decision))
	if decision == "" {
		if env.Result.Pass {
			decision = "pass"
		} else {
			decision = "fail"
		}
	}
	switch decision {
	case "pass", "retry", "reclassify", "fail":
	default:
		return domain.Evaluation{}, fmt.Errorf("unsupported evaluator decision: %s", decision)
	}
	score := normalizeScore(env.Result.Score)
	pass := env.Result.Pass
	if decision != "pass" {
		pass = false
	}
	if decision == "pass" && !pass {
		pass = true
	}
	retryable := env.Result.Retryable
	if decision == "retry" {
		retryable = true
	}
	return domain.Evaluation{
		Pass:            pass,
		Score:           score,
		Feedback:        strings.TrimSpace(env.Result.Feedback),
		Decision:        decision,
		PolicyViolation: env.Result.PolicyViolation,
		Retryable:       retryable,
		Metadata: map[string]any{
			"reason":   strings.TrimSpace(env.Result.Reason),
			"protocol": evaluatorProtocolV1,
		},
	}, nil
}

func fallbackEvaluator(raw string) domain.Evaluation {
	text := strings.ToUpper(strings.TrimSpace(raw))
	if strings.Contains(text, "RECLASSIFY") {
		return domain.Evaluation{
			Pass:      false,
			Score:     0.25,
			Feedback:  strings.TrimSpace(raw),
			Decision:  "reclassify",
			Retryable: true,
		}
	}
	if strings.Contains(text, "RETRY") {
		return domain.Evaluation{
			Pass:      false,
			Score:     0.20,
			Feedback:  strings.TrimSpace(raw),
			Decision:  "retry",
			Retryable: true,
		}
	}
	if strings.Contains(text, "FAIL") {
		return domain.Evaluation{
			Pass:      false,
			Score:     0.10,
			Feedback:  strings.TrimSpace(raw),
			Decision:  "fail",
			Retryable: false,
		}
	}
	return domain.Evaluation{
		Pass:      true,
		Score:     0.95,
		Feedback:  strings.TrimSpace(raw),
		Decision:  "pass",
		Retryable: false,
	}
}

func fromWorkerMetadata(meta map[string]any) (domain.Evaluation, bool) {
	if len(meta) == 0 {
		return domain.Evaluation{}, false
	}
	var toolFailed bool
	if v, ok := meta["tool_failed"].(bool); ok && v {
		toolFailed = true
	}
	errText, _ := meta["error"].(string)
	if !toolFailed && strings.TrimSpace(errText) == "" {
		return domain.Evaluation{}, false
	}
	reason := strings.TrimSpace(errText)
	if reason == "" {
		reason = "tool execution failed"
	}
	if looksLikePolicyViolation(reason) {
		return domain.Evaluation{
			Pass:            false,
			Score:           0.15,
			Feedback:        reason,
			Decision:        "retry",
			PolicyViolation: true,
			Retryable:       true,
			Metadata:        map[string]any{"reason": "policy_violation"},
		}, true
	}
	if looksLikeDomainMismatch(reason) {
		return domain.Evaluation{
			Pass:      false,
			Score:     0.20,
			Feedback:  reason,
			Decision:  "reclassify",
			Retryable: true,
			Metadata:  map[string]any{"reason": "domain_mismatch"},
		}, true
	}
	return domain.Evaluation{
		Pass:      false,
		Score:     0.10,
		Feedback:  reason,
		Decision:  "retry",
		Retryable: true,
		Metadata:  map[string]any{"reason": "tool_failed"},
	}, true
}

func looksLikePolicyViolation(reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(lower, "policy violation") ||
		strings.Contains(lower, "forbidden tool") ||
		strings.Contains(lower, "required tool") ||
		strings.Contains(lower, "outside step policy") ||
		strings.Contains(lower, "cannot call tool")
}

func looksLikeDomainMismatch(reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(lower, "domain mismatch") ||
		strings.Contains(lower, "not compatible with domain")
}

func normalizeScore(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
