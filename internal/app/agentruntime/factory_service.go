package agentruntime

import (
	"context"
	"errors"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/agentruntime"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
	port "go-sse-skeleton/internal/port/agentruntime"
)

type FactoryService interface {
	Build(ctx context.Context, cmd BuildRuntimeCommand) (BuildRuntimeResult, error)
}

type factoryService struct {
	configRepo       port.AgentConfigRepository
	toolRegistry     port.ToolRegistry
	chatClientReg    port.ChatClientRegistry
	memoryBuilder    port.MemoryBuilder
	graphBuilder     port.GraphBuilder
	runtimeAssembler RuntimeAssembler

	// Shared dependencies injected for consistency with project-wide constructor style.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
}

func NewFactoryService(
	configRepo port.AgentConfigRepository,
	toolRegistry port.ToolRegistry,
	chatClientReg port.ChatClientRegistry,
	memoryBuilder port.MemoryBuilder,
	graphBuilder port.GraphBuilder,
	runtimeAssembler RuntimeAssembler,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
) (FactoryService, error) {
	if configRepo == nil || toolRegistry == nil || chatClientReg == nil || memoryBuilder == nil || graphBuilder == nil || runtimeAssembler == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in runtime factory")
	}
	return &factoryService{
		configRepo:       configRepo,
		toolRegistry:     toolRegistry,
		chatClientReg:    chatClientReg,
		memoryBuilder:    memoryBuilder,
		graphBuilder:     graphBuilder,
		runtimeAssembler: runtimeAssembler,
		messageStore:     messageStore,
		eventBus:         eventBus,
		sseNotifier:      sseNotifier,
		llmClient:        llmClient,
	}, nil
}

func (s *factoryService) Build(ctx context.Context, cmd BuildRuntimeCommand) (BuildRuntimeResult, error) {
	if cmd.AgentID == "" || cmd.SessionID == "" {
		return BuildRuntimeResult{}, domain.ErrInvalidInput
	}

	cfg, err := s.configRepo.GetAgentConfig(ctx, cmd.AgentID)
	if err != nil {
		return BuildRuntimeResult{}, err
	}

	fixed, err := s.toolRegistry.FixedTools(ctx)
	if err != nil {
		return BuildRuntimeResult{}, err
	}
	optional, err := s.toolRegistry.OptionalTools(ctx)
	if err != nil {
		// Keep main path resilient: optional tool load failures should not block runtime creation.
		slog.Warn("load optional tools failed", "agentID", cmd.AgentID, "sessionID", cmd.SessionID, "err", err)
		optional = nil
	}
	allTools := append(fixed, optional...)
	selectedTools := domain.FilterToolsByAllowList(allTools, cfg.AllowedTools)
	toolNames := make([]string, 0, len(selectedTools))
	for _, t := range selectedTools {
		toolNames = append(toolNames, t.Name)
	}

	knowledge := make([]domain.KnowledgeBase, 0)
	if len(cfg.AllowedKBs) > 0 {
		kbList, kbErr := s.configRepo.ListKnowledgeBases(ctx, cfg.AllowedKBs)
		if kbErr != nil {
			// Keep runtime build resilient: missing KB metadata should not block runtime startup.
			slog.Warn("list allowed knowledge bases failed", "agentID", cmd.AgentID, "sessionID", cmd.SessionID, "err", kbErr)
		} else {
			knowledge = kbList
		}
	}

	chatClient, err := s.chatClientReg.GetByModel(ctx, cfg.Model)
	if err != nil {
		// Keep Java-like fallback behavior: if model-specific bean is missing, use global llm client.
		slog.Warn("chat client registry fallback to default llm client", "agentID", cmd.AgentID, "model", cfg.Model, "err", err)
		chatClient = s.llmClient
	}

	maxMessages := cfg.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 30
	}
	memory, err := s.memoryBuilder.Build(ctx, cmd.SessionID, maxMessages)
	if err != nil {
		return BuildRuntimeResult{}, err
	}

	agentID := cfg.AgentID
	if agentID == "" {
		agentID = cmd.AgentID
	}

	graph, err := s.graphBuilder.Build(ctx, port.GraphBuildInput{
		AgentID:      agentID,
		SessionID:    cmd.SessionID,
		SystemPrompt: cfg.SystemPrompt,
		Tools:        toolNames,
		Knowledge:    knowledge,
		ModelClient:  chatClient,
		Memory:       memory,
	})
	if err != nil {
		return BuildRuntimeResult{}, err
	}

	runtime, err := s.runtimeAssembler.Assemble(ctx, RuntimeAssembleInput{
		AgentID:   agentID,
		SessionID: cmd.SessionID,
		Graph:     graph,
	})
	if err != nil {
		return BuildRuntimeResult{}, err
	}
	var stream assistantStreamNotifier
	if typed, ok := any(s.sseNotifier).(assistantStreamNotifier); ok {
		stream = typed
	}
	runtime = wrapRuntimeWithExecution(runtime, executionHooks{
		messageStore: s.messageStore,
		stream:       stream,
		sessionID:    cmd.SessionID,
		maxMessages:  maxMessages,
	})
	runtime = wrapRuntimeWithLifecycle(runtime, lifecycleHooks{
		eventBus:    s.eventBus,
		sseNotifier: s.sseNotifier,
		agentID:     agentID,
		sessionID:   cmd.SessionID,
	})

	return BuildRuntimeResult{
		Runtime: runtime,
		Spec: domain.RuntimeSpec{
			AgentID:      agentID,
			SessionID:    cmd.SessionID,
			Model:        cfg.Model,
			SystemPrompt: cfg.SystemPrompt,
		},
	}, nil
}
