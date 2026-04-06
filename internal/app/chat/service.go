package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/chat"
	"go-sse-skeleton/internal/port/cache"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	"go-sse-skeleton/internal/port/repository"
	sseport "go-sse-skeleton/internal/port/sse"
)

type Service interface {
	CreateFromCommand(ctx context.Context, cmd CreateMessageCommand) (string, error)
	CreateInternal(ctx context.Context, msg *domain.Message) (string, error)
	Append(ctx context.Context, messageID string, appendContent string) error
	ListBySession(ctx context.Context, sessionID string) ([]*domain.Message, error)
	ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error)
	Update(ctx context.Context, messageID string, cmd UpdateMessageCommand) error
	Delete(ctx context.Context, messageID string) error
}

type service struct {
	store          repository.ChatMessageStore
	memoryCache    cache.ChatMemoryCache
	eventPublisher queue.ChatEventPublisher
	sseNotifier    sseport.MessageNotifier
	llmClient      llm.Client
}

func NewService(
	store repository.ChatMessageStore,
	memoryCache cache.ChatMemoryCache,
	eventPublisher queue.ChatEventPublisher,
	sseNotifier sseport.MessageNotifier,
	llmClient llm.Client,
) (Service, error) {
	if store == nil || memoryCache == nil || eventPublisher == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in chat service")
	}
	return &service{
		store:           store,
		memoryCache:     memoryCache,
		eventPublisher: eventPublisher,
		sseNotifier:     sseNotifier,
		llmClient:       llmClient,
	}, nil
}

func (s *service) CreateFromCommand(ctx context.Context, cmd CreateMessageCommand) (string, error) {
	if cmd.SessionID == "" || cmd.Role == "" {
		return "", newInvalidInputError("sessionId and role are required")
	}
	msg := &domain.Message{
		SessionID: cmd.SessionID,
		Role:      cmd.Role,
		Content:   cmd.Content,
		Metadata:  cmd.Metadata,
	}

	id, err := s.store.Create(ctx, msg)
	if err != nil {
		return "", wrapAppError(err)
	}

	if cmd.Role == domain.RoleUser {
		// No explicit DB transaction here, aligned with Java behavior:
		// once message persistence succeeds, cache/event failures should not rollback creation.
		if err = s.memoryCache.Invalidate(ctx, cmd.SessionID); err != nil {
			slog.Warn("memory cache invalidate failed", "sessionID", cmd.SessionID, "err", err)
		}
		if err = s.eventPublisher.PublishChatEvent(ctx, cmd.AgentID, cmd.SessionID, cmd.Content); err != nil {
			// keep Java behavior: do not rollback message creation for event publish failure
			slog.Warn("publish chat event failed", "agentID", cmd.AgentID, "sessionID", cmd.SessionID, "err", err)
		}
	}

	return id, nil
}

func (s *service) CreateInternal(ctx context.Context, msg *domain.Message) (string, error) {
	if msg == nil {
		return "", newInvalidInputError("message is nil")
	}
	id, err := s.store.Create(ctx, msg)
	if err != nil {
		return "", wrapAppError(err)
	}
	return id, nil
}

func (s *service) Append(ctx context.Context, messageID string, appendContent string) error {
	if messageID == "" || appendContent == "" {
		return newInvalidInputError("chatMessageId and appendContent are required")
	}

	if appender, ok := s.store.(repository.ChatMessageAppender); ok {
		// Prefer SQL-level atomic append to avoid lost updates under concurrent DELTA writes.
		return wrapAppError(appender.AppendContent(ctx, messageID, appendContent))
	}

	existing, err := s.store.GetByID(ctx, messageID)
	if err != nil {
		return wrapAppError(err)
	}
	if existing == nil {
		return wrapAppError(domain.ErrNotFound)
	}

	if err = existing.Append(appendContent); err != nil {
		return wrapAppError(err)
	}

	// TODO: remove fallback once all repositories implement ChatMessageAppender for atomic append.
	return wrapAppError(s.store.Update(ctx, existing))
}

func (s *service) ListBySession(ctx context.Context, sessionID string) ([]*domain.Message, error) {
	if sessionID == "" {
		return nil, newInvalidInputError("sessionId is required")
	}
	out, err := s.store.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, wrapAppError(err)
	}
	return out, nil
}

func (s *service) ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error) {
	if sessionID == "" || limit <= 0 {
		return nil, newInvalidInputError("sessionId is required and limit must be > 0")
	}
	out, err := s.store.ListRecentBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, wrapAppError(err)
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, messageID string, cmd UpdateMessageCommand) error {
	if messageID == "" {
		return newInvalidInputError("chatMessageId is required")
	}

	existing, err := s.store.GetByID(ctx, messageID)
	if err != nil {
		return wrapAppError(err)
	}
	if existing == nil {
		return wrapAppError(domain.ErrNotFound)
	}

	if cmd.Content != nil {
		existing.Content = *cmd.Content
	}
	if cmd.Metadata != nil {
		existing.Metadata = cmd.Metadata
	}

	return wrapAppError(s.store.Update(ctx, existing))
}

func (s *service) Delete(ctx context.Context, messageID string) error {
	if messageID == "" {
		return newInvalidInputError("chatMessageId is required")
	}

	existing, err := s.store.GetByID(ctx, messageID)
	if err != nil {
		return wrapAppError(err)
	}
	if existing == nil {
		return wrapAppError(fmt.Errorf("%w: %s", domain.ErrNotFound, messageID))
	}

	return wrapAppError(s.store.Delete(ctx, messageID))
}
