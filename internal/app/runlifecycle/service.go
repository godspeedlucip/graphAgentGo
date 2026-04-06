package runlifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	registry        RuntimeRegistry
	retryPolicy     RetryPolicy

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
	opts ...Option,
) (Service, error) {
	if runtimeProvider == nil || runRepo == nil || guard == nil || orchestrator == nil || clock == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in run lifecycle service")
	}
	svc := &service{
		runtimeProvider: runtimeProvider,
		runRepo:         runRepo,
		guard:           guard,
		orchestrator:    orchestrator,
		clock:           clock,
		registry:        NewRuntimeRegistry(),
		retryPolicy:     defaultRetryPolicy(),
		messageStore:    messageStore,
		eventBus:        eventBus,
		sseNotifier:     sseNotifier,
		llmClient:       llmClient,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if svc.registry == nil {
		svc.registry = NewRuntimeRegistry()
	}
	if svc.retryPolicy.MaxAttempts <= 0 {
		svc.retryPolicy = defaultRetryPolicy()
	}
	return svc, nil
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

	if err = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusInit, domain.StatusRunning, meta); err != nil {
		return RunResult{}, err
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if cmd.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cmd.Timeout)
	} else {
		runCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	s.registry.Register(cmd.RunID, cancel)
	defer s.registry.Unregister(cmd.RunID)

	var (
		out       port.RuntimeOutput
		runErr    error
		hasDelta  bool
		attempted int
	)
	maxAttempts := s.retryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attempted = attempt
		runtime, getErr := s.runtimeProvider.GetRuntime(runCtx, cmd.AgentID, cmd.SessionID)
		if getErr != nil {
			failMeta := mergeMeta(meta, map[string]any{
				"error": getErr.Error(),
			})
			_ = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusRunning, domain.StatusFailed, failMeta)
			return RunResult{RunID: cmd.RunID, Status: domain.StatusFailed, ErrorCode: "RUNTIME_BUILD_FAILED", ErrorMsg: getErr.Error()}, getErr
		}
		out, runErr = runtime.Run(runCtx, port.RuntimeInput{
			RunID:     cmd.RunID,
			AgentID:   cmd.AgentID,
			SessionID: cmd.SessionID,
			UserInput: cmd.UserInput,
			Metadata:  meta,
			AppendOutput: func(appendCtx context.Context, delta string) error {
				if delta == "" {
					return nil
				}
				if appendErr := s.runRepo.AppendOutput(appendCtx, cmd.RunID, delta); appendErr != nil {
					return appendErr
				}
				hasDelta = true
				s.orchestrator.EmitDelta(appendCtx, cmd.RunID, cmd.SessionID, delta, meta)
				return nil
			},
		})
		if runErr == nil {
			break
		}
		status := mapRunErrorToStatus(runCtx, runErr)
		if !s.retryPolicy.ShouldRetry(status, attempt, hasDelta) {
			break
		}
		if sleepErr := s.retryPolicy.Sleep(runCtx); sleepErr != nil {
			runErr = sleepErr
			break
		}
	}
	if attempted > 1 {
		slog.Info("run retry attempts finished", "runID", cmd.RunID, "attempts", attempted)
	}

	if runErr != nil {
		status := mapRunErrorToStatus(runCtx, runErr)
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

	if out.Text != "" && !hasDelta {
		if err = s.runRepo.AppendOutput(ctx, cmd.RunID, out.Text); err != nil {
			failMeta := mergeMeta(meta, map[string]any{
				"error":      fmt.Sprintf("append output failed: %v", err),
				"finishedAt": s.clock.Now().Format(time.RFC3339Nano),
			})
			_ = s.orchestrator.Transit(ctx, cmd.RunID, domain.StatusRunning, domain.StatusFailed, failMeta)
			return RunResult{RunID: cmd.RunID, Status: domain.StatusFailed, ErrorCode: "OUTPUT_APPEND_FAILED", ErrorMsg: err.Error()}, err
		}
	}
	finalOutput := out.Text
	if hasDelta && finalOutput == "" {
		if rec, getErr := s.runRepo.Get(ctx, cmd.RunID); getErr == nil && rec != nil {
			finalOutput = rec.Output
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

	return RunResult{RunID: cmd.RunID, Status: domain.StatusDone, Output: finalOutput}, nil
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
	_ = s.registry.Cancel(cmd.RunID)
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
