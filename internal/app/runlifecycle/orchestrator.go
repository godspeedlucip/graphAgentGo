package runlifecycle

import (
	"context"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
	port "go-sse-skeleton/internal/port/runlifecycle"
)

type Orchestrator interface {
	Transit(ctx context.Context, runID string, from domain.Status, to domain.Status, metadata map[string]any) error
	EmitDelta(ctx context.Context, runID string, sessionID string, delta string, metadata map[string]any)
}

type orchestrator struct {
	repo      port.RunRepository
	publisher port.EventPublisher
	notifier  port.SSENotifier
	clock     port.Clock
}

func NewOrchestrator(
	repo port.RunRepository,
	publisher port.EventPublisher,
	notifier port.SSENotifier,
	clock port.Clock,
) (Orchestrator, error) {
	if repo == nil || publisher == nil || notifier == nil || clock == nil {
		return nil, domain.ErrInvalidInput
	}
	return &orchestrator{repo: repo, publisher: publisher, notifier: notifier, clock: clock}, nil
}

func (o *orchestrator) Transit(ctx context.Context, runID string, from domain.Status, to domain.Status, metadata map[string]any) error {
	if runID == "" {
		return domain.ErrInvalidInput
	}
	if !domain.CanTransit(from, to) && from != to {
		return domain.ErrInvalidTransition
	}
	if err := o.repo.UpdateStatus(ctx, runID, to, metadata); err != nil {
		return err
	}

	evt := domain.LifecycleEvent{
		RunID:      runID,
		Status:     to,
		OccurredAt: o.clock.Now(),
		Metadata:   metadata,
	}
	if err := o.publisher.PublishLifecycle(ctx, evt); err != nil {
		// Keep shell path resilient: side-channel event failures should not block run status update.
		slog.Warn("publish lifecycle skipped", "runID", runID, "status", to, "err", err)
	}

	sessionID, _ := metadataString(metadata, "sessionId")
	switch to {
	case domain.StatusRunning:
		if err := o.notifier.NotifyStarted(ctx, runID, sessionID); err != nil {
			slog.Warn("notify run started skipped", "runID", runID, "sessionID", sessionID, "err", err)
		}
	case domain.StatusDone:
		if err := o.notifier.NotifyDone(ctx, runID, sessionID); err != nil {
			slog.Warn("notify run done skipped", "runID", runID, "sessionID", sessionID, "err", err)
		}
	case domain.StatusFailed, domain.StatusTimeout, domain.StatusCanceled:
		reason, _ := metadataString(metadata, "error")
		if reason == "" {
			reason = string(to)
		}
		if err := o.notifier.NotifyFailed(ctx, runID, sessionID, reason); err != nil {
			slog.Warn("notify run failed skipped", "runID", runID, "sessionID", sessionID, "status", to, "err", err)
		}
	}

	return nil
}

func (o *orchestrator) EmitDelta(ctx context.Context, runID string, sessionID string, delta string, metadata map[string]any) {
	if runID == "" || delta == "" {
		return
	}
	evtMeta := map[string]any{
		"runId":     runID,
		"sessionId": sessionID,
		"delta":     delta,
		"stage":     "delta",
	}
	for k, v := range metadata {
		evtMeta[k] = v
	}
	if err := o.publisher.PublishLifecycle(ctx, domain.LifecycleEvent{
		RunID:      runID,
		SessionID:  sessionID,
		Status:     domain.StatusRunning,
		OccurredAt: o.clock.Now(),
		Metadata:   evtMeta,
	}); err != nil {
		slog.Warn("publish lifecycle delta skipped", "runID", runID, "sessionID", sessionID, "err", err)
	}
	if err := o.notifier.NotifyDelta(ctx, runID, sessionID, delta); err != nil {
		slog.Warn("notify run delta skipped", "runID", runID, "sessionID", sessionID, "err", err)
	}
}

func metadataString(metadata map[string]any, key string) (string, bool) {
	if len(metadata) == 0 {
		return "", false
	}
	v, ok := metadata[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}
