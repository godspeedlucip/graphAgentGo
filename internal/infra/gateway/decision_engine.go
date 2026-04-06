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
		if !hitAudience(rule, req) {
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

func hitAudience(rule domain.Rule, req domain.Request) bool {
	if len(rule.UserIDs) > 0 && !containsFold(rule.UserIDs, headerValue(req.Header, "x-user-id")) {
		return false
	}
	if len(rule.TenantIDs) > 0 && !containsFold(rule.TenantIDs, headerValue(req.Header, "x-tenant-id")) {
		return false
	}
	if len(rule.AgentIDs) > 0 && !containsFold(rule.AgentIDs, headerValue(req.Header, "x-agent-id")) {
		return false
	}
	return true
}

func containsFold(items []string, target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func headerValue(h map[string]string, key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
