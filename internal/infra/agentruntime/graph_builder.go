package agentruntime

import (
	"context"
	"errors"
	"fmt"

	port "go-sse-skeleton/internal/port/agentruntime"
)

type promptGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type compiledGraph struct {
	agentID      string
	sessionID    string
	systemPrompt string
	toolNames    []string
	kbIDs        []string
	model        promptGenerator
}

func (g *compiledGraph) Execute(ctx context.Context) error {
	if g == nil {
		return errors.New("nil compiled graph")
	}
	if g.model == nil {
		return errors.New("nil graph model")
	}

	// TODO: map SupervisorNode/GraphEngine DAG once node-runtime contracts are finalized.
	_, err := g.model.Generate(ctx, g.buildPrompt())
	return err
}

func (g *compiledGraph) buildPrompt() string {
	return fmt.Sprintf("agent=%s session=%s tools=%v kbs=%v system=%s",
		g.agentID, g.sessionID, g.toolNames, g.kbIDs, g.systemPrompt)
}

type PlaceholderGraphBuilder struct{}

func NewPlaceholderGraphBuilder() *PlaceholderGraphBuilder {
	return &PlaceholderGraphBuilder{}
}

func (b *PlaceholderGraphBuilder) Build(_ context.Context, in port.GraphBuildInput) (port.GraphRuntime, error) {
	if in.AgentID == "" || in.SessionID == "" {
		return nil, errors.New("invalid graph build input")
	}
	model, ok := in.ModelClient.(promptGenerator)
	if !ok || model == nil {
		return nil, errors.New("model client does not implement prompt generator")
	}

	kbIDs := make([]string, 0, len(in.Knowledge))
	for _, kb := range in.Knowledge {
		if kb.ID != "" {
			kbIDs = append(kbIDs, kb.ID)
		}
	}

	return &compiledGraph{
		agentID:      in.AgentID,
		sessionID:    in.SessionID,
		systemPrompt: in.SystemPrompt,
		toolNames:    append([]string(nil), in.Tools...),
		kbIDs:        kbIDs,
		model:        model,
	}, nil
}

