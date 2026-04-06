package agentruntime

import (
	"context"
	"errors"

	domain "go-sse-skeleton/internal/domain/agentruntime"
)

type AgentConfigRepository struct {
	configs map[string]domain.AgentConfig
	kbs     map[string]domain.KnowledgeBase
}

func NewAgentConfigRepository(configs []domain.AgentConfig, kbs []domain.KnowledgeBase) *AgentConfigRepository {
	cfgMap := make(map[string]domain.AgentConfig, len(configs))
	for _, c := range configs {
		if c.AgentID == "" {
			continue
		}
		cfgMap[c.AgentID] = c
	}
	kbMap := make(map[string]domain.KnowledgeBase, len(kbs))
	for _, kb := range kbs {
		if kb.ID == "" {
			continue
		}
		kbMap[kb.ID] = kb
	}
	return &AgentConfigRepository{configs: cfgMap, kbs: kbMap}
}

func (r *AgentConfigRepository) GetAgentConfig(_ context.Context, agentID string) (*domain.AgentConfig, error) {
	if agentID == "" {
		return nil, errors.New("empty agent id")
	}
	cfg, ok := r.configs[agentID]
	if !ok {
		return nil, errors.New("agent config not found")
	}
	copied := cfg
	return &copied, nil
}

func (r *AgentConfigRepository) ListKnowledgeBases(_ context.Context, kbIDs []string) ([]domain.KnowledgeBase, error) {
	if len(kbIDs) == 0 {
		return nil, nil
	}
	out := make([]domain.KnowledgeBase, 0, len(kbIDs))
	for _, id := range kbIDs {
		if kb, ok := r.kbs[id]; ok {
			out = append(out, kb)
		}
	}
	return out, nil
}

