package sse

import (
	"context"
	"errors"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/sse"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	"go-sse-skeleton/internal/port/repository"
	sseport "go-sse-skeleton/internal/port/sse"
)

type Service struct {
	repo  repository.ChatMessageRepository
	queue queue.EventPublisher
	hub   sseport.Hub
	llm   llm.Client
}

func NewService(repo repository.ChatMessageRepository, queue queue.EventPublisher, hub sseport.Hub, llm llm.Client) (*Service, error) {
	if repo == nil || queue == nil || hub == nil || llm == nil {
		return nil, errors.New("nil dependency in sse service")
	}
	return &Service{repo: repo, queue: queue, hub: hub, llm: llm}, nil
}

func (s *Service) Send(ctx context.Context, sessionID string, msg domain.Message) error {
	if sessionID == "" {
		return domain.ErrInvalidInput
	}
	if err := msg.Validate(); err != nil {
		return err
	}

	err := s.hub.Publish(ctx, sessionID, msg)
	if err == nil {
		return nil
	}

	// Keep behavior compatible with Java SseServiceImpl:
	// no active client or transient disconnect should not break main agent flow.
	if errors.Is(err, domain.ErrClientNotFound) || errors.Is(err, domain.ErrClientDisconnected) {
		slog.Warn("skip sse push", "sessionID", sessionID, "type", msg.Type, "err", err)
		return nil
	}
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
	msg := domain.Message{
		Type: domain.TypeAIGeneratedContentStart,
		Payload: domain.Payload{
			// Keep Java payload shape: message object may be present.
			// TODO: fill Payload.Message using persisted assistant placeholder once chat message module is integrated.
			Done: boolPtr(false),
		},
		Metadata: domain.Metadata{ChatMessageID: chatMessageID},
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