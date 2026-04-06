package sse

import (
	"context"
	"errors"
	"log/slog"
	"time"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/sse"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	"go-sse-skeleton/internal/port/repository"
	sseport "go-sse-skeleton/internal/port/sse"
)

type Service struct {
	repo      repository.ChatMessageRepository
	queue     queue.EventPublisher
	hub       sseport.Hub
	llm       llm.Client
	telemetry Telemetry
}

func NewService(repo repository.ChatMessageRepository, queue queue.EventPublisher, hub sseport.Hub, llm llm.Client, opts ...ServiceOption) (*Service, error) {
	if repo == nil || queue == nil || hub == nil || llm == nil {
		return nil, errors.New("nil dependency in sse service")
	}
	svc := &Service{
		repo:      repo,
		queue:     queue,
		hub:       hub,
		llm:       llm,
		telemetry: NewNoopTelemetry(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if svc.telemetry == nil {
		svc.telemetry = NewNoopTelemetry()
	}
	return svc, nil
}

func (s *Service) Send(ctx context.Context, sessionID string, msg domain.Message) error {
	begin := time.Now()
	if sessionID == "" {
		s.telemetry.RecordSend(ctx, string(msg.Type), "invalid_input", false, time.Since(begin))
		return domain.ErrInvalidInput
	}
	if err := msg.Validate(); err != nil {
		s.telemetry.RecordSend(ctx, string(msg.Type), "invalid_message", false, time.Since(begin))
		return err
	}

	err := s.hub.Publish(ctx, sessionID, msg)
	if err == nil {
		s.telemetry.RecordSend(ctx, string(msg.Type), "ok", true, time.Since(begin))
		return nil
	}

	// Keep behavior compatible with Java SseServiceImpl:
	// no active client or transient disconnect should not break main agent flow.
	if errors.Is(err, domain.ErrClientNotFound) || errors.Is(err, domain.ErrClientDisconnected) {
		s.telemetry.RecordSend(ctx, string(msg.Type), classifySendErr(err), true, time.Since(begin))
		slog.Warn("skip sse push", "sessionID", sessionID, "type", msg.Type, "err", err)
		return nil
	}
	s.telemetry.RecordSend(ctx, string(msg.Type), classifySendErr(err), false, time.Since(begin))
	return err
}

func (s *Service) SendGeneratedContent(ctx context.Context, sessionID string, chatMessageID string, message any) error {
	msg := domain.Message{
		Type: domain.TypeAIGeneratedContent,
		Payload: domain.Payload{
			Message: message,
		},
		Metadata: domain.Metadata{ChatMessageID: chatMessageID},
	}
	return s.Send(ctx, sessionID, msg)
}

func (s *Service) StartAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error {
	begin := time.Now()
	createdID, err := s.repo.Create(ctx, sessionID, string(chatdomain.RoleAssistant), "")
	if err != nil {
		s.telemetry.RecordAssistantPlaceholderCreate(ctx, sessionID, "persist_failed", false, time.Since(begin))
		return err
	}
	s.telemetry.RecordAssistantPlaceholderCreate(ctx, sessionID, "ok", true, time.Since(begin))
	// Keep current API shape for compatibility. chatMessageID argument is ignored here,
	// and the persisted ID becomes the source of truth for START metadata.
	// TODO: wire runtime call chain to consume this persisted ID for DELTA/END without relying on external input.
	msg := domain.Message{
		Type: domain.TypeAIGeneratedContentStart,
		Payload: domain.Payload{
			// Keep Java payload shape: include assistant placeholder message in START event payload.
			Message: map[string]any{
				"id":        createdID,
				"sessionId": sessionID,
				"role":      string(chatdomain.RoleAssistant),
				"content":   "",
			},
			Done: boolPtr(false),
		},
		Metadata: domain.Metadata{ChatMessageID: createdID},
	}
	return s.Send(ctx, sessionID, msg)
}

func (s *Service) AppendAssistantDelta(ctx context.Context, sessionID string, chatMessageID string, delta string) error {
	msg := domain.Message{
		Type: domain.TypeAIGeneratedContentDelta,
		Payload: domain.Payload{
			DeltaContent: delta,
		},
		Metadata: domain.Metadata{ChatMessageID: chatMessageID},
	}
	return s.Send(ctx, sessionID, msg)
}

func (s *Service) EndAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error {
	msg := domain.Message{
		Type: domain.TypeAIGeneratedContentEnd,
		Payload: domain.Payload{
			Done: boolPtr(true),
		},
		Metadata: domain.Metadata{ChatMessageID: chatMessageID},
	}
	return s.Send(ctx, sessionID, msg)
}

func (s *Service) NotifyDone(ctx context.Context, sessionID string) error {
	msg := domain.Message{
		Type: domain.TypeAIDone,
		Payload: domain.Payload{
			Done: boolPtr(true),
		},
	}
	return s.Send(ctx, sessionID, msg)
}

func boolPtr(v bool) *bool {
	return &v
}

func classifySendErr(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, domain.ErrClientNotFound):
		return "client_not_found"
	case errors.Is(err, domain.ErrClientDisconnected):
		return "client_disconnected"
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, domain.ErrInvalidMessage):
		return "invalid_message"
	default:
		return "publish_failed"
	}
}
