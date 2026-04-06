package orchestration

import (
	"context"
	"testing"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type fixedRule struct {
	name     string
	priority int
	verdict  domain.PolicyVerdict
	matched  bool
}

func (r fixedRule) Name() string { return r.name }

func (r fixedRule) Priority() int { return r.priority }

func (r fixedRule) Decide(context.Context, domain.GraphState, domain.Evaluation) (domain.PolicyVerdict, bool, error) {
	return r.verdict, r.matched, nil
}

func TestStaticPolicyComposablePriority(t *testing.T) {
	t.Parallel()

	p := NewStaticPolicy(WithPolicyRules(
		fixedRule{name: "low", priority: 10, verdict: domain.PolicyVerdict{Continue: false, Reason: "low"}, matched: true},
		fixedRule{name: "high", priority: 100, verdict: domain.PolicyVerdict{Continue: false, Reason: "high"}, matched: true},
	))

	verdict, err := p.Decide(context.Background(), domain.GraphState{}, domain.Evaluation{})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if verdict.Reason != "high" {
		t.Fatalf("expected high priority rule, got %+v", verdict)
	}
}

func TestStaticPolicyConfigurableThreshold(t *testing.T) {
	t.Parallel()

	p := NewStaticPolicy(WithPolicyMinPassScore(0.75))
	verdict, err := p.Decide(context.Background(), domain.GraphState{StepIndex: 0, MaxSteps: 5}, domain.Evaluation{
		Pass: true, Score: 0.80, Decision: "pass",
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if verdict.Final != domain.FinalStatusDone || verdict.Reason != "goal_reached" {
		t.Fatalf("unexpected verdict: %+v", verdict)
	}
}

