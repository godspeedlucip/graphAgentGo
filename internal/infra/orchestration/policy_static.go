package orchestration

import (
	"context"
	"sort"

	domain "go-sse-skeleton/internal/domain/orchestration"
)

type PolicyRule interface {
	Name() string
	Priority() int
	Decide(ctx context.Context, state domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, bool, error)
}

type StaticPolicyOption func(*StaticPolicy)

type StaticPolicy struct {
	minPassScore float64
	rules        []PolicyRule
}

func NewStaticPolicy(opts ...StaticPolicyOption) *StaticPolicy {
	p := &StaticPolicy{
		minPassScore: 0.90,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	if len(p.rules) == 0 {
		p.rules = defaultPolicyRules(p.minPassScore)
	}
	sort.SliceStable(p.rules, func(i, j int) bool {
		return p.rules[i].Priority() > p.rules[j].Priority()
	})
	return p
}

func WithPolicyMinPassScore(v float64) StaticPolicyOption {
	return func(p *StaticPolicy) {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		p.minPassScore = v
	}
}

func WithPolicyRules(rules ...PolicyRule) StaticPolicyOption {
	return func(p *StaticPolicy) {
		filtered := make([]PolicyRule, 0, len(rules))
		for _, r := range rules {
			if r != nil {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) > 0 {
			p.rules = filtered
		}
	}
}

func (p *StaticPolicy) Decide(ctx context.Context, state domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, error) {
	for _, rule := range p.rules {
		verdict, matched, err := rule.Decide(ctx, state, eval)
		if err != nil {
			return domain.PolicyVerdict{}, err
		}
		if matched {
			if !verdict.Continue && verdict.Reason == "" {
				verdict.Reason = rule.Name()
			}
			return verdict, nil
		}
	}
	return domain.PolicyVerdict{Continue: true}, nil
}

func defaultPolicyRules(minPassScore float64) []PolicyRule {
	return []PolicyRule{
		terminalFailureRule{},
		stepLimitRule{},
		goalReachedRule{threshold: minPassScore},
	}
}

type terminalFailureRule struct{}

func (terminalFailureRule) Name() string   { return "terminal_failure" }
func (terminalFailureRule) Priority() int  { return 100 }
func (terminalFailureRule) Decide(_ context.Context, _ domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, bool, error) {
	switch eval.Decision {
	case "retry", "reclassify":
		return domain.PolicyVerdict{Continue: true, Reason: eval.Decision}, true, nil
	case "fail":
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusFailed, Reason: "evaluation_failed"}, true, nil
	}
	if !eval.Pass && !eval.Retryable {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusFailed, Reason: "evaluation_failed"}, true, nil
	}
	return domain.PolicyVerdict{}, false, nil
}

type stepLimitRule struct{}

func (stepLimitRule) Name() string  { return "step_limit" }
func (stepLimitRule) Priority() int { return 80 }
func (stepLimitRule) Decide(_ context.Context, state domain.GraphState, _ domain.Evaluation) (domain.PolicyVerdict, bool, error) {
	if state.StepIndex+1 >= state.MaxSteps {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusDone, Reason: "step_limit"}, true, nil
	}
	return domain.PolicyVerdict{}, false, nil
}

type goalReachedRule struct {
	threshold float64
}

func (r goalReachedRule) Name() string  { return "goal_reached" }
func (r goalReachedRule) Priority() int { return 60 }
func (r goalReachedRule) Decide(_ context.Context, _ domain.GraphState, eval domain.Evaluation) (domain.PolicyVerdict, bool, error) {
	if eval.Pass && eval.Score >= r.threshold {
		return domain.PolicyVerdict{Continue: false, Final: domain.FinalStatusDone, Reason: "goal_reached"}, true, nil
	}
	return domain.PolicyVerdict{}, false, nil
}
