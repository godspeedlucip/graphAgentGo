package orchestration

import (
	"context"
	"errors"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/orchestration"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	port "go-sse-skeleton/internal/port/orchestration"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
)

type Service interface {
	Execute(ctx context.Context, cmd ExecuteGraphCommand) (ExecuteGraphResult, error)
}

type service struct {
	engine    port.GraphEngine
	traceRepo port.TraceRepository
	publisher port.EventPublisher
	notifier  port.StreamNotifier

	// Shared dependencies injected for consistency with project-wide constructor style.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
}

func NewService(
	engine port.GraphEngine,
	traceRepo port.TraceRepository,
	publisher port.EventPublisher,
	notifier port.StreamNotifier,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
) (Service, error) {
	if engine == nil || traceRepo == nil || publisher == nil || notifier == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in orchestration service")
	}
	return &service{
		engine:       engine,
		traceRepo:    traceRepo,
		publisher:    publisher,
		notifier:     notifier,
		messageStore: messageStore,
		eventBus:     eventBus,
		sseNotifier:  sseNotifier,
		llmClient:    llmClient,
	}, nil
}

func (s *service) Execute(ctx context.Context, cmd ExecuteGraphCommand) (ExecuteGraphResult, error) {
	if cmd.Input.RunID == "" || cmd.Input.AgentID == "" || cmd.Input.SessionID == "" {
		return ExecuteGraphResult{}, domain.ErrInvalidInput
	}
	in := cmd.Input
	in.MaxSteps = domain.NormalizeMaxSteps(in.MaxSteps)

	result, err := s.engine.Execute(ctx, in)
	if err != nil {
		// Side-channel event publication should not mask execution error.
		_ = s.publisher.PublishGraphEvent(ctx, "orchestration.failed", map[string]any{
			"runId":     in.RunID,
			"agentId":   in.AgentID,
			"sessionId": in.SessionID,
			"error":     err.Error(),
		})
		return ExecuteGraphResult{}, err
	}

	for _, step := range result.Trace {
		if appendErr := s.traceRepo.AppendStep(ctx, in.RunID, step); appendErr != nil {
			// Keep core execution result success path unaffected by trace persistence failures.
			slog.Warn("append orchestration trace skipped", "runID", in.RunID, "step", step.StepIndex, "err", appendErr)
		}
		if notifyErr := s.notifier.NotifyStep(ctx, in.SessionID, step); notifyErr != nil {
			slog.Warn("notify orchestration step skipped", "runID", in.RunID, "step", step.StepIndex, "err", notifyErr)
		}
	}

	if pubErr := s.publisher.PublishGraphEvent(ctx, "orchestration.final", map[string]any{
		"runId":     in.RunID,
		"agentId":   in.AgentID,
		"sessionId": in.SessionID,
		"status":    result.Status,
		"reason":    result.Reason,
		"stepsUsed": result.StepsUsed,
	}); pubErr != nil {
		slog.Warn("publish orchestration final skipped", "runID", in.RunID, "err", pubErr)
	}

	if notifyErr := s.notifier.NotifyFinal(ctx, in.SessionID, result); notifyErr != nil {
		slog.Warn("notify orchestration final skipped", "runID", in.RunID, "err", notifyErr)
	}

	return ExecuteGraphResult{Output: result}, nil
}
