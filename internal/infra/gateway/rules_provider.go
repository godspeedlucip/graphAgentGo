package gateway

import (
	"context"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type StaticRulesProvider struct {
	rules domain.Rules
}

func NewStaticRulesProvider(rules domain.Rules) *StaticRulesProvider {
	return &StaticRulesProvider{rules: rules}
}

func (p *StaticRulesProvider) Current(_ context.Context) (domain.Rules, error) {
	return p.rules, nil
}