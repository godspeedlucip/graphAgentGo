package agentruntime

import (
	"context"
	"errors"
	"sync"
	"testing"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/agentruntime"
	port "go-sse-skeleton/internal/port/agentruntime"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	sseport "go-sse-skeleton/internal/port/sse"
)

type fakeConfigRepo struct {
	cfg domain.AgentConfig
}

func (r fakeConfigRepo) GetAgentConfig(context.Context, string) (*domain.AgentConfig, error) {
	copied := r.cfg
	return &copied, nil
}

func (r fakeConfigRepo) ListKnowledgeBases(context.Context, []string) ([]domain.KnowledgeBase, error) {
	return nil, nil
}

type fakeToolRegistry struct {
	fixed       []domain.ToolDef
	optional    []domain.ToolDef
	optionalErr error
}

func (r fakeToolRegistry) FixedTools(context.Context) ([]domain.ToolDef, error) { return r.fixed, nil }

func (r fakeToolRegistry) OptionalTools(context.Context) ([]domain.ToolDef, error) {
	return r.optional, r.optionalErr
}

type fakeChatClientRegistry struct {
	client port.ChatClient
	err    error
}

func (r fakeChatClientRegistry) GetByModel(context.Context, string) (port.ChatClient, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.client, nil
}

type fakeMemoryBuilder struct{}

func (fakeMemoryBuilder) Build(context.Context, string, int) (port.Memory, error) { return struct{}{}, nil }

type fakeGraph struct {
	tokens []string
}

func (g *fakeGraph) Execute(ctx context.Context) error {
	hooks, ok := port.ExecutionHooksFromContext(ctx)
	if !ok {
		return nil
	}
	for _, t := range g.tokens {
		if hooks.AppendDelta != nil {
			_ = hooks.AppendDelta(ctx, t)
		}
		if hooks.CollectDelta != nil {
			hooks.CollectDelta(t)
		}
	}
	return nil
}

type fakeGraphBuilder struct {
	lastInput port.GraphBuildInput
	graph     port.GraphRuntime
}

func (b *fakeGraphBuilder) Build(_ context.Context, in port.GraphBuildInput) (port.GraphRuntime, error) {
	b.lastInput = in
	if b.graph != nil {
		return b.graph, nil
	}
	return &fakeGraph{tokens: []string{"Hello", " ", "world"}}, nil
}

type fakeRuntime struct {
	agentID   string
	sessionID string
	graph     port.GraphRuntime
}

func (r *fakeRuntime) Run(ctx context.Context) error { return r.graph.Execute(ctx) }

func (r *fakeRuntime) AgentID() string { return r.agentID }

func (r *fakeRuntime) SessionID() string { return r.sessionID }

type fakeRuntimeAssembler struct{}

func (a fakeRuntimeAssembler) Assemble(_ context.Context, in RuntimeAssembleInput) (port.Runtime, error) {
	return &fakeRuntime{agentID: in.AgentID, sessionID: in.SessionID, graph: in.Graph}, nil
}

type fakeMessageStore struct {
	mu          sync.Mutex
	recentCalls int
	createCalls int
	updateCalls int
	updated     *chatdomain.Message
}

func (s *fakeMessageStore) Create(_ context.Context, msg *chatdomain.Message) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCalls++
	return "assistant-1", nil
}

func (s *fakeMessageStore) GetByID(context.Context, string) (*chatdomain.Message, error) { return nil, nil }

func (s *fakeMessageStore) ListBySession(context.Context, string) ([]*chatdomain.Message, error) { return nil, nil }

func (s *fakeMessageStore) ListRecentBySession(context.Context, string, int) ([]*chatdomain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recentCalls++
	return []*chatdomain.Message{{SessionID: "s1", Role: chatdomain.RoleUser, Content: "hi"}}, nil
}

func (s *fakeMessageStore) Update(_ context.Context, msg *chatdomain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCalls++
	copied := *msg
	s.updated = &copied
	return nil
}

func (s *fakeMessageStore) Delete(context.Context, string) error { return nil }

type flakyEventBus struct{}

func (flakyEventBus) Publish(context.Context, string, any) error { return errors.New("queue failed") }

type flakySSE struct {
	startCalls int
	deltaCalls int
	endCalls   int
	doneCalls  int
}

func (s *flakySSE) StartAssistantStream(context.Context, string, string) error {
	s.startCalls++
	return errors.New("sse start failed")
}

func (s *flakySSE) AppendAssistantDelta(context.Context, string, string, string) error {
	s.deltaCalls++
	return errors.New("sse delta failed")
}

func (s *flakySSE) EndAssistantStream(context.Context, string, string) error {
	s.endCalls++
	return errors.New("sse end failed")
}

func (s *flakySSE) NotifyDone(context.Context, string) error {
	s.doneCalls++
	return errors.New("sse done failed")
}

var _ sseport.MessageNotifier = (*flakySSE)(nil)

type defaultLLM struct{}

func (defaultLLM) Generate(context.Context, string) (string, error) { return "fallback", nil }

var _ llm.Client = defaultLLM{}
var _ queue.EventPublisher = flakyEventBus{}
var _ repo.ChatMessageStore = (*fakeMessageStore)(nil)

func TestFactoryBuildRuntimeContractAndSideChannelResilience(t *testing.T) {
	t.Parallel()

	graphBuilder := &fakeGraphBuilder{}
	store := &fakeMessageStore{}
	sseNotifier := &flakySSE{}

	svc, err := NewFactoryService(
		fakeConfigRepo{cfg: domain.AgentConfig{
			AgentID:      "agent-1",
			Model:        "gpt-x",
			SystemPrompt: "you are helpful",
			AllowedTools: []string{"fixed_tool"},
			MaxMessages:  5,
		}},
		fakeToolRegistry{
			fixed:       []domain.ToolDef{{Name: "fixed_tool", Kind: "tool"}},
			optionalErr: errors.New("optional registry down"),
		},
		fakeChatClientRegistry{err: errors.New("model missing")},
		fakeMemoryBuilder{},
		graphBuilder,
		fakeRuntimeAssembler{},
		store,
		flakyEventBus{},
		sseNotifier,
		defaultLLM{},
	)
	if err != nil {
		t.Fatalf("new factory service: %v", err)
	}

	result, err := svc.Build(context.Background(), BuildRuntimeCommand{AgentID: "agent-1", SessionID: "s1"})
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if result.Runtime == nil {
		t.Fatal("runtime should not be nil")
	}
	if result.Spec.AgentID != "agent-1" || result.Spec.SessionID != "s1" || result.Spec.Model != "gpt-x" {
		t.Fatalf("unexpected runtime spec: %+v", result.Spec)
	}
	// model fallback should pass default llm client to graph builder when registry misses.
	if _, ok := graphBuilder.lastInput.ModelClient.(defaultLLM); !ok {
		t.Fatalf("expected model fallback to default llm client, got %T", graphBuilder.lastInput.ModelClient)
	}
	// optional tool load failure should degrade, fixed tool still available in graph input.
	if len(graphBuilder.lastInput.Tools) != 1 || graphBuilder.lastInput.Tools[0] != "fixed_tool" {
		t.Fatalf("unexpected selected tools: %+v", graphBuilder.lastInput.Tools)
	}

	runErr := result.Runtime.Run(context.Background())
	if runErr != nil {
		t.Fatalf("runtime run should not fail on side-channel errors: %v", runErr)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.recentCalls == 0 || store.createCalls == 0 || store.updateCalls == 0 {
		t.Fatalf("expected memory/message store closed-loop calls, got recent=%d create=%d update=%d", store.recentCalls, store.createCalls, store.updateCalls)
	}
	if store.updated == nil || store.updated.Content != "Hello world" || store.updated.Role != chatdomain.RoleAssistant {
		t.Fatalf("unexpected assistant persisted message: %+v", store.updated)
	}
	if sseNotifier.startCalls == 0 || sseNotifier.deltaCalls == 0 || sseNotifier.endCalls == 0 || sseNotifier.doneCalls == 0 {
		t.Fatalf("expected start/delta/end/done sse calls, got start=%d delta=%d end=%d done=%d",
			sseNotifier.startCalls, sseNotifier.deltaCalls, sseNotifier.endCalls, sseNotifier.doneCalls)
	}
}
