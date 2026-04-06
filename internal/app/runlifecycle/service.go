package runlifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	port "go-sse-skeleton/internal/port/runlifecycle"
	"go-sse-skeleton/internal/port/sse"
)

type Service interface {
	Start(ctx context.Context, cmd StartRunCommand) (RunResult, error)
	Cancel(ctx context.Context, cmd CancelRunCommand) error
	Get(ctx context.Context, runID string) (RunResult, error)
}

type service struct {
	runtimeProvider port.RuntimeProvider
	runRepo         port.RunRepository
	guard           port.IdempotencyGuard
	orchestrator    Orchestrator
	clock           port.Clock

	// Shared dependencies injected for consistency with project-wide constructor style.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
}

func NewService(
	runtimeProvider port.RuntimeProvider,
	runRepo port.RunRepository,
	guard port.IdempotencyGuard,
	orchestrator Orchestrator,
	clock port.Clock,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
) (Service, error) {
	if runtimeProvider == nil || runRepo == nil || guard == nil || orchestrator == nil || clock == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in run lifecycle service")
	}
	return &service{
		runtimeProvider: runtimeProvider,
		runRepo:         runRepo,
		guard:           guard,
		orchestrator:    orchestrator,
		clock:           clock,
		messageStore:    messageStore,
		eventBus:        eventBus,
		sseNotifier:     sseNotifier,
		llmClient:       llmClient,
	}, nil
}

func (s *service) Start(ctx context.Context, cmd StartRunCommand) (RunResult, error) {
	if cmd.RunID == "" || cmd.AgentID == "" || cmd.SessionID == "" {
		return RunResult{}, domain.ErrInvalidInput
	}

	ok, err := s.guard.Acquire(ctx, cmd.RunID, 5*time.Minute)
	if err != nil {
		return RunResult{}, err
	}
	if !ok {
		return RunResult{}, domain.ErrRunBusy
	}
	defer func() { _ = s.guard.Release(context.Background(), cmd.RunID) }()

	meta := cloneMeta(cmd.Metadata)
	meta["sessionId"] = cmd.SessionID
	meta["agentId"] = cmd.AgentID
	meta["runId"] = cmd.RunID
	meta["startedAt"] = s.clock.Now().Format(time.RFC3339Nano)

	if err = s.runRepo.Create(ctx, domain.RunRecord{
		RunID:     cmd.RunID,
		AgentID:   cmd.AgentID,
		SessionID: cmd.SessionID,
		Status:    domain.StatusInit,
		Input:     cmd.UserInput,
		Metadata:  meta,
		StartedAt: s.clock.Now(),
	}); err != nil {
		if errors.Is(err, domain.ErrRunAlreadyExists) {
			return RunResult{}, err
		}
		return RunResult{}, err
	}

	runtime, err := s.runtimeProvider.GetRuntime(ctx, cmd.AgentID, cmd.SessionID)
	if err != nil {
		failMeta := mergeMeta(meta, map[string]any{
			"error": err.Error(),
		})
		_ = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusInit, domain.StatusFailed, failMeta)
		return RunResult{RunID: cmd.RunID, Status: domain.StatusFailed, ErrorCode: "RUNTIME_BUILD_FAILED", ErrorMsg: err.Error()}, err
	}

	if err = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusInit, domain.StatusRunning, meta); err != nil {
		return RunResult{}, err
	}

	out, runErr := runtime.Run(ctx, port.RuntimeInput{
		RunID:     cmd.RunID,
		AgentID:   cmd.AgentID,
		SessionID: cmd.SessionID,
		UserInput: cmd.UserInput,
		Metadata:  meta,
	})

	if runErr != nil {
		status := mapRunErrorToStatus(ctx, runErr)
		failMeta := mergeMeta(meta, map[string]any{
			"error":      runErr.Error(),
			"finishedAt": s.clock.Now().Format(time.RFC3339Nano),
		})
		_ = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusRunning, status, failMeta)
		return RunResult{
			RunID:     cmd.RunID,
			Status:    status,
			ErrorCode: statusErrorCode(status),
			ErrorMsg:  runErr.Error(),
		}, runErr
	}

	if out.Text != "" {
		if err = s.runRepo.AppendOutput(ctx, cmd.RunID, out.Text); err != nil {
			failMeta := mergeMeta(meta, map[string]any{
				"error":      fmt.Sprintf("append output failed: %v", err),
				"finishedAt": s.clock.Now().Format(time.RFC3339Nano),
			})
			_ = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusRunning, domain.StatusFailed, failMeta)
			return RunResult{RunID: cmd.RunID, Status: domain.StatusFailed, ErrorCode: "OUTPUT_APPEND_FAILED", ErrorMsg: err.Error()}, err
		}
	}

	doneMeta := mergeMeta(meta, out.Metadata)
	doneMeta["finishedAt"] = s.clock.Now().Format(time.RFC3339Nano)
	if err = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusRunning, domain.StatusDone, doneMeta); err != nil {
		return RunResult{}, err
	}

	// Keep compatibility with existing SSE notifier abstraction used in other modules.
	// Side-channel failure should not break successful run result.
	_ = s.sseNotifier.NotifyDone(ctx, cmd.SessionID)

	return RunResult{RunID: cmd.RunID, Status: domain.StatusDone, Output: out.Text}, nil
}

func (s *service) Cancel(ctx context.Context, cmd CancelRunCommand) error {
	if cmd.RunID == "" {
		return domain.ErrInvalidInput
	}
	rec, err := s.runRepo.Get(ctx, cmd.RunID)
	if err != nil {
		return err
	}
	if rec == nil {
		return domain.ErrRunNotFound
	}
	if domain.IsTerminal(rec.Status) {
		return nil
	}

	meta := mergeMeta(rec.Metadata, map[string]any{
		"cause":      cmd.Cause,
		"sessionId":  rec.SessionID,
		"agentId":    rec.AgentID,
		"finishedAt": s.clock.Now().Format(time.RFC3339Nano),
	})
	if err = s.orchestrator.Transit(ctx, cmd.RunID, rec.Status, domain.StatusCanceled, meta); err != nil {
		return err
	}
	// TODO: propagate cancellation signal to runtime execution goroutine when runtime cancellation registry is introduced.
	return nil
}

func (s *service) Get(ctx context.Context, runID string) (RunResult, error) {
	if runID == "" {
		return RunResult{}, domain.ErrInvalidInput
	}
	rec, err := s.runRepo.Get(ctx, runID)
	if err != nil {
		return RunResult{}, err
	}
	if rec == nil {
		return RunResult{}, domain.ErrRunNotFound
	}
	return RunResult{
		RunID:     rec.RunID,
		Status:    rec.Status,
		Output:    rec.Output,
		ErrorCode: rec.ErrorCode,
		ErrorMsg:  rec.ErrorMsg,
	}, nil
}

func mapRunErrorToStatus(ctx context.Context, runErr error) domain.Status {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(runErr, context.DeadlineExceeded) {
		return domain.StatusTimeout
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(runErr, context.Canceled) {
		return domain.StatusCanceled
	}
	return domain.StatusFailed
}

func statusErrorCode(status domain.Status) string {
	switch status {
	case domain.StatusTimeout:
		return "RUN_TIMEOUT"
	case domain.StatusCanceled:
		return "RUN_CANCELED"
	default:
		return "RUN_FAILED"
	}
}

func cloneMeta(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeMeta(base map[string]any, ext map[string]any) map[string]any {
	out := cloneMeta(base)
	for k, v := range ext {
		out[k] = v
	}
	return out
}
