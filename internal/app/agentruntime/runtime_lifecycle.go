package agentruntime

import (
	"context"
	"fmt"
	"log/slog"

	"go-sse-skeleton/internal/port/queue"
	port "go-sse-skeleton/internal/port/agentruntime"
	"go-sse-skeleton/internal/port/sse"
)

const runtimeLifecycleTopic = "agent.runtime.lifecycle"

type lifecycleHooks struct {
	eventBus    queue.EventPublisher
	sseNotifier sse.MessageNotifier
	agentID     string
	sessionID   string
}

type lifecycleRuntime struct {
	base  port.Runtime
	hooks lifecycleHooks
}

func wrapRuntimeWithLifecycle(base port.Runtime, hooks lifecycleHooks) port.Runtime {
	if base == nil {
		return nil
	}
	if hooks.eventBus == nil && hooks.sseNotifier == nil {
		return base
	}
	return &lifecycleRuntime{base: base, hooks: hooks}
}

func (r *lifecycleRuntime) Run(ctx context.Context) (err error) {
	r.publish(ctx, "start", nil)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("runtime panic: %v", rec)
		}
		if err != nil {
			r.publish(ctx, "failed", map[string]any{"error": err.Error()})
			return
		}
		r.publish(ctx, "done", nil)
		if r.hooks.sseNotifier != nil {
			// TODO: align with Java incremental SSE semantics (start/delta/end events) once graph stream API is available.
			if notifyErr := r.hooks.sseNotifier.NotifyDone(ctx, r.SessionID()); notifyErr != nil {
				// Side-channel failure should not fail the main runtime result.
				slog.Warn("notify runtime done skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "err", notifyErr)
			}
		}
	}()

	err = r.base.Run(ctx)
	return err
}

func (r *lifecycleRuntime) AgentID() string {
	return r.base.AgentID()
}

func (r *lifecycleRuntime) SessionID() string {
	return r.base.SessionID()
}

func (r *lifecycleRuntime) publish(ctx context.Context, stage string, extra map[string]any) {
	if r.hooks.eventBus == nil {
		return
	}
	payload := map[string]any{
		"agentId":   r.AgentID(),
		"sessionId": r.SessionID(),
		"stage":     stage,
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := r.hooks.eventBus.Publish(ctx, runtimeLifecycleTopic, payload); err != nil {
		// Keep runtime path resilient: lifecycle metrics/events are best-effort.
		slog.Warn("publish runtime lifecycle skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "stage", stage, "err", err)
	}
}
