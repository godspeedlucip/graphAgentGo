package gateway

import (
	"context"
	"hash/fnv"
	"strings"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type RuleDecisionEngine struct{}

func NewRuleDecisionEngine() *RuleDecisionEngine {
	return &RuleDecisionEngine{}
}

func (e *RuleDecisionEngine) Decide(_ context.Context, req domain.Request, rules domain.Rules) (domain.Decision, error) {
	method := strings.TrimSpace(req.Method)
	path := strings.TrimSpace(req.Path)
	if method == "" || path == "" {
		return domain.Decision{}, domain.ErrInvalidInput
	}

	for _, rule := range rules.Items {
		if !rule.Enabled {
			continue
		}
		if rule.Method != "" && !strings.EqualFold(rule.Method, method) {
			continue
		}
		if rule.PathPrefix != "" && !strings.HasPrefix(path, rule.PathPrefix) {
			continue
		}
		if rule.Target != domain.TargetJava && rule.Target != domain.TargetGo {
			continue
		}
		if !hitTrafficRatio(rule, req) {
			continue
		}
		return domain.Decision{Target: rule.Target, Reason: rule.Name}, nil
	}

	if rules.DefaultTarget != domain.TargetJava && rules.DefaultTarget != domain.TargetGo {
		return domain.Decision{}, domain.ErrNoRouteMatched
	}
	return domain.Decision{Target: rules.DefaultTarget, Reason: "default"}, nil
}

func hitTrafficRatio(rule domain.Rule, req domain.Request) bool {
	// Keep compatibility: unspecified ratio defaults to full traffic.
	if rule.TrafficRatio <= 0 {
		return true
	}
	if rule.TrafficRatio >= 100 {
		return true
	}

	seed := req.Method + "|" + req.Path
	if req.Header != nil {
		// Prefer sticky key to reduce route flapping for same client/session.
		if v := strings.TrimSpace(req.Header["x-request-id"]); v != "" {
			seed = v
		} else if v := strings.TrimSpace(req.Header["x-session-id"]); v != "" {
			seed = v
		}
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	bucket := int(h.Sum32() % 100)
	return bucket < rule.TrafficRatio
}
