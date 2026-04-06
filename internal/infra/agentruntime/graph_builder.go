package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	port "go-sse-skeleton/internal/port/agentruntime"
)

type promptGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type streamPromptGenerator interface {
	GenerateStream(ctx context.Context, prompt string, onToken func(token string) error) error
}

type graphState struct {
	prompt       string
	output       string
	toolDecision string
	reflection   string
}

type graphNode interface {
	Name() string
	Run(ctx context.Context, g *compiledGraph, state *graphState) error
}

type compiledGraph struct {
	agentID      string
	sessionID    string
	systemPrompt string
	toolNames    []string
	kbIDs        []string
	model        promptGenerator
	dag          map[string][]string
	nodes        map[string]graphNode
}

func (g *compiledGraph) Execute(ctx context.Context) error {
	if g == nil {
		return errors.New("nil compiled graph")
	}
	if g.model == nil {
		return errors.New("nil graph model")
	}

	state := &graphState{}
	indegree := map[string]int{}
	for name := range g.nodes {
		indegree[name] = 0
	}
	for _, edges := range g.dag {
		for _, next := range edges {
			indegree[next]++
		}
	}
	queue := make([]string, 0)
	for name, degree := range indegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	executed := 0
	for len(queue) > 0 {
		nodeName := queue[0]
		queue = queue[1:]
		node := g.nodes[nodeName]
		if node == nil {
			continue
		}
		if err := node.Run(ctx, g, state); err != nil {
			return fmt.Errorf("node %s failed: %w", node.Name(), err)
		}
		executed++
		for _, next := range g.dag[nodeName] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if executed != len(g.nodes) {
		return errors.New("invalid dag: cycle detected")
	}
	return nil
}

type planNode struct{}

func (n planNode) Name() string { return "plan" }

func (n planNode) Run(ctx context.Context, g *compiledGraph, state *graphState) error {
	hooks, _ := port.ExecutionHooksFromContext(ctx)
	state.prompt = buildPrompt(g, hooks.RecentMessages)
	return nil
}

type actNode struct{}

func (n actNode) Name() string { return "act" }

func (n actNode) Run(ctx context.Context, g *compiledGraph, state *graphState) error {
	hooks, _ := port.ExecutionHooksFromContext(ctx)
	var out strings.Builder
	appendToken := func(token string) error {
		if hooks.CollectDelta != nil {
			hooks.CollectDelta(token)
		}
		if hooks.AppendDelta != nil {
			_ = hooks.AppendDelta(ctx, token)
		}
		_, _ = out.WriteString(token)
		return nil
	}

	if streamModel, ok := any(g.model).(streamPromptGenerator); ok {
		if err := streamModel.GenerateStream(ctx, state.prompt, appendToken); err != nil {
			return err
		}
		state.output = out.String()
		return nil
	}

	result, err := g.model.Generate(ctx, state.prompt)
	if err != nil {
		return err
	}
	for _, token := range tokenizeForStream(result) {
		_ = appendToken(token)
	}
	state.output = out.String()
	return nil
}

type toolNode struct{}

func (n toolNode) Name() string { return "tool" }

func (n toolNode) Run(_ context.Context, g *compiledGraph, state *graphState) error {
	if len(g.toolNames) == 0 {
		state.toolDecision = "no_tool"
		return nil
	}
	// TODO: bind dynamic tool discovery + permission policy + retry/circuit-breaker strategy.
	state.toolDecision = "tool_path_reserved"
	return nil
}

type reflectNode struct{}

func (n reflectNode) Name() string { return "reflect" }

func (n reflectNode) Run(_ context.Context, _ *compiledGraph, state *graphState) error {
	if strings.TrimSpace(state.output) == "" {
		state.reflection = "empty_output"
		return nil
	}
	// Minimal reflection hook to reserve place for future critique/replan loop.
	state.reflection = "ok"
	return nil
}

func tokenizeForStream(s string) []string {
	if s == "" {
		return nil
	}
	var tokens []string
	var current strings.Builder
	for _, r := range s {
		current.WriteRune(r)
		if r == ' ' || r == '\n' {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func buildPrompt(g *compiledGraph, recent []*chatdomain.Message) string {
	var history strings.Builder
	for _, msg := range recent {
		if msg == nil || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		history.WriteString("[")
		history.WriteString(string(msg.Role))
		history.WriteString("] ")
		history.WriteString(msg.Content)
		history.WriteString("\n")
	}
	return fmt.Sprintf("agent=%s session=%s tools=%v kbs=%v system=%s history=%s",
		g.agentID, g.sessionID, g.toolNames, g.kbIDs, g.systemPrompt, history.String())
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

	nodes := map[string]graphNode{
		"plan":    planNode{},
		"act":     actNode{},
		"tool":    toolNode{},
		"reflect": reflectNode{},
	}
	dag := map[string][]string{
		"plan":    {"act"},
		"act":     {"tool"},
		"tool":    {"reflect"},
		"reflect": {},
	}

	return &compiledGraph{
		agentID:      in.AgentID,
		sessionID:    in.SessionID,
		systemPrompt: in.SystemPrompt,
		toolNames:    append([]string(nil), in.Tools...),
		kbIDs:        kbIDs,
		model:        model,
		dag:          dag,
		nodes:        nodes,
	}, nil
}
