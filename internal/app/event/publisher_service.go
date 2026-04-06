package event

import (
	"context"
	"errors"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/event"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
	port "go-sse-skeleton/internal/port/event"
)

type PublisherService interface {
	PublishChatEvent(ctx context.Context, cmd PublishChatEventCommand) error
}

type publisherService struct {
	publisher port.Publisher

	// Shared dependencies injected for consistency with project-wide constructor style.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
	observer     port.Observer
}

func NewPublisherService(
	publisher port.Publisher,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
	opts ...PublisherOption,
) (PublisherService, error) {
	if publisher == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in event publisher service")
	}
	svc := &publisherService{
		publisher:    publisher,
		messageStore: messageStore,
		eventBus:     eventBus,
		sseNotifier:  sseNotifier,
		llmClient:    llmClient,
		observer:     noopObserver{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc, nil
}

func (s *publisherService) PublishChatEvent(ctx context.Context, cmd PublishChatEventCommand) error {
	if err := cmd.Event.Validate(); err != nil {
		return err
	}
	if err := s.publisher.PublishChatEvent(ctx, cmd.Event); err != nil {
		// Keep Java-like behavior: event dispatch failures should not block request main path.
		s.observer.RecordFailed("chat_event", "publish")
		slog.Warn("publish chat event skipped", "eventID", cmd.Event.EventID, "agentID", cmd.Event.AgentID, "sessionID", cmd.Event.SessionID, "err", err)
		return nil
	}
	s.observer.RecordPublished("chat_event")
	return nil
}

var _ PublisherService = (*publisherService)(nil)
var _ = domain.ChatEvent{}
