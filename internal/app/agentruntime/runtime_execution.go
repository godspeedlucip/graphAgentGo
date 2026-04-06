package agentruntime

import (
	"bytes"
	"context"
	"log/slog"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	port "go-sse-skeleton/internal/port/agentruntime"
	repo "go-sse-skeleton/internal/port/repository"
)

type assistantStreamNotifier interface {
	StartAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error
	AppendAssistantDelta(ctx context.Context, sessionID string, chatMessageID string, delta string) error
	EndAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error
}

type executionHooks struct {
	messageStore repo.ChatMessageStore
	stream       assistantStreamNotifier
	sessionID    string
	maxMessages  int
}

type executionRuntime struct {
	base  port.Runtime
	hooks executionHooks
}

func wrapRuntimeWithExecution(base port.Runtime, hooks executionHooks) port.Runtime {
	if base == nil {
		return nil
	}
	if hooks.messageStore == nil {
		return base
	}
	if hooks.maxMessages <= 0 {
		hooks.maxMessages = 30
	}
	return &executionRuntime{base: base, hooks: hooks}
}

func (r *executionRuntime) Run(ctx context.Context) error {
	recent, err := r.hooks.messageStore.ListRecentBySession(ctx, r.SessionID(), r.hooks.maxMessages)
	if err != nil {
		// Memory read fallback: runtime main chain should continue even when recent context fetch fails.
		slog.Warn("runtime recent memory read skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "err", err)
		recent = nil
	}

	assistantID, err := r.hooks.messageStore.Create(ctx, &chatdomain.Message{
		SessionID: r.SessionID(),
		Role:      chatdomain.RoleAssistant,
		Content:   "",
	})
	if err != nil {
		return err
	}

	if r.hooks.stream != nil {
		if streamErr := r.hooks.stream.StartAssistantStream(ctx, r.SessionID(), assistantID); streamErr != nil {
			// SSE stream is side-channel: failures should not break runtime main path.
			slog.Warn("runtime sse start skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "chatMessageID", assistantID, "err", streamErr)
		}
	}

	var buf bytes.Buffer
	runCtx := port.WithExecutionHooks(ctx, port.ExecutionHooks{
		RecentMessages:    recent,
		AssistantMessageID: assistantID,
		CollectDelta: func(delta string) {
			_, _ = buf.WriteString(delta)
		},
		AppendDelta: func(appendCtx context.Context, delta string) error {
			if r.hooks.stream == nil {
				return nil
			}
			if appendErr := r.hooks.stream.AppendAssistantDelta(appendCtx, r.SessionID(), assistantID, delta); appendErr != nil {
				slog.Warn("runtime sse delta skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "chatMessageID", assistantID, "err", appendErr)
			}
			return nil
		},
	})

	runErr := r.base.Run(runCtx)
	if endErr := r.hooks.messageStore.Update(ctx, &chatdomain.Message{
		ID:        assistantID,
		SessionID: r.SessionID(),
		Role:      chatdomain.RoleAssistant,
		Content:   buf.String(),
	}); endErr != nil {
		return endErr
	}
	if r.hooks.stream != nil {
		if streamErr := r.hooks.stream.EndAssistantStream(ctx, r.SessionID(), assistantID); streamErr != nil {
			slog.Warn("runtime sse end skipped", "agentID", r.AgentID(), "sessionID", r.SessionID(), "chatMessageID", assistantID, "err", streamErr)
		}
	}
	return runErr
}

func (r *executionRuntime) AgentID() string {
	return r.base.AgentID()
}

func (r *executionRuntime) SessionID() string {
	return r.base.SessionID()
}
