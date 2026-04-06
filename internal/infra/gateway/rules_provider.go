package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type StaticRulesProvider struct {
	rules domain.Rules
}

func NewStaticRulesProvider(rules domain.Rules) *StaticRulesProvider {
	return &StaticRulesProvider{rules: normalizeRules(rules)}
}

func (p *StaticRulesProvider) Current(_ context.Context) (domain.Rules, error) {
	return p.rules, nil
}

type AtomicRulesProvider struct {
	current atomic.Value
}

func NewAtomicRulesProvider(initial domain.Rules) *AtomicRulesProvider {
	p := &AtomicRulesProvider{}
	p.current.Store(normalizeRules(initial))
	return p
}

func (p *AtomicRulesProvider) Current(_ context.Context) (domain.Rules, error) {
	v := p.current.Load()
	if v == nil {
		return domain.Rules{}, errors.New("gateway rules not initialized")
	}
	rules, ok := v.(domain.Rules)
	if !ok {
		return domain.Rules{}, errors.New("gateway rules type assertion failed")
	}
	return rules, nil
}

func (p *AtomicRulesProvider) Update(rules domain.Rules) {
	p.current.Store(normalizeRules(rules))
}

type FileRulesProvider struct {
	path    string
	atomic  *AtomicRulesProvider
	mu      sync.Mutex
	lastMod int64
}

func NewFileRulesProvider(path string, fallback domain.Rules) (*FileRulesProvider, error) {
	if path == "" {
		return nil, errors.New("empty rules file path")
	}
	return &FileRulesProvider{
		path:   path,
		atomic: NewAtomicRulesProvider(fallback),
	}, nil
}

func (p *FileRulesProvider) Current(ctx context.Context) (domain.Rules, error) {
	_ = ctx
	if err := p.tryReload(); err != nil {
		// Fail-open with last known good rules.
		return p.atomic.Current(context.Background())
	}
	return p.atomic.Current(context.Background())
}

func (p *FileRulesProvider) tryReload() error {
	info, err := os.Stat(p.path)
	if err != nil {
		return err
	}
	mod := info.ModTime().UnixNano()
	p.mu.Lock()
	defer p.mu.Unlock()
	if mod == p.lastMod {
		return nil
	}
	raw, err := os.ReadFile(p.path)
	if err != nil {
		return err
	}
	var rules domain.Rules
	if err = json.Unmarshal(raw, &rules); err != nil {
		return err
	}
	p.atomic.Update(rules)
	p.lastMod = mod
	return nil
}

func normalizeRules(rules domain.Rules) domain.Rules {
	if rules.DefaultTarget != domain.TargetJava && rules.DefaultTarget != domain.TargetGo {
		rules.DefaultTarget = domain.TargetJava
	}
	if rules.IdempotencyHeader == "" {
		rules.IdempotencyHeader = "Idempotency-Key"
	}
	return rules
}
