package gateway

import (
	"context"
	"net/http"
	"testing"

	domain "go-sse-skeleton/internal/domain/gateway"
)

func TestRuleDecisionEngineAudienceMatch(t *testing.T) {
	t.Parallel()

	engine := NewRuleDecisionEngine()
	rules := domain.Rules{
		DefaultTarget: domain.TargetJava,
		Items: []domain.Rule{
			{
				Name:       "user-gray-go",
				PathPrefix: "/api/chat",
				Method:     http.MethodPost,
				Target:     domain.TargetGo,
				UserIDs:    []string{"u-1"},
				Enabled:    true,
			},
		},
	}

	decision, err := engine.Decide(context.Background(), domain.Request{
		Method: http.MethodPost,
		Path:   "/api/chat/append",
		Header: map[string]string{"X-User-Id": "u-1"},
	}, rules)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Target != domain.TargetGo {
		t.Fatalf("expected go target, got %+v", decision)
	}
}

func TestRuleDecisionEngineAudienceMissFallsBackDefault(t *testing.T) {
	t.Parallel()

	engine := NewRuleDecisionEngine()
	rules := domain.Rules{
		DefaultTarget: domain.TargetJava,
		Items: []domain.Rule{
			{
				Name:       "tenant-gray-go",
				PathPrefix: "/api/chat",
				Method:     http.MethodPost,
				Target:     domain.TargetGo,
				TenantIDs:  []string{"t-1"},
				Enabled:    true,
			},
		},
	}

	decision, err := engine.Decide(context.Background(), domain.Request{
		Method: http.MethodPost,
		Path:   "/api/chat/append",
		Header: map[string]string{"X-Tenant-Id": "t-2"},
	}, rules)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Target != domain.TargetJava || decision.Reason != "default" {
		t.Fatalf("unexpected default decision: %+v", decision)
	}
}
