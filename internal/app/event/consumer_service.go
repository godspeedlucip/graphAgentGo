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

type AgentRunner interface {
	RunByEvent(ctx context.Context, evt domain.ChatEvent) error
}

type ConsumerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Handle(ctx context.Context, evt domain.ChatEvent) error
}

type consumerService struct {
	subscriber port.Subscriber
	dispatcher port.Dispatcher
	runner     AgentRunner

	// Shared dependencies injected for consistency with project-wide constructor style.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
}

func NewConsumerService(
	subscriber port.Subscriber,
	dispatcher port.Dispatcher,
	runner AgentRunner,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
) (ConsumerService, error) {
	if subscriber == nil || dispatcher == nil || runner == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in event consumer service")
	}
	return &consumerService{
		subscriber:   subscriber,
		dispatcher:   dispatcher,
		runner:       runner,
		messageStore: messageStore,
		eventBus:     eventBus,
		sseNotifier:  sseNotifier,
		llmClient:    llmClient,
	}, nil
}

func (s *consumerService) Start(ctx context.Context) error {
	if err := s.dispatcher.Start(ctx); err != nil {
		return err
	}
	return s.subscriber.SubscribeChatEvent(ctx, s)
}

func (s *consumerService) Stop(ctx context.Context) error {
	return s.dispatcher.Stop(ctx)
}

func (s *consumerService) Handle(ctx context.Context, evt domain.ChatEvent) error {
	if err := evt.Validate(); err != nil {
		return err
	}
	// Async equivalence of @Async/@EventListener:
	// submit job to dispatcher and return immediately without waiting for runner completion.
	return s.dispatcher.Submit(ctx, func(jobCtx context.Context) error {
		if runErr := s.runner.RunByEvent(jobCtx, evt); runErr != nil {
			slog.Error("async event handling failed", "eventID", evt.EventID, "agentID", evt.AgentID, "sessionID", evt.SessionID, "err", runErr)
			return runErr
		}
		return nil
	})
}

var _ port.ChatEventHandler = (*consumerService)(nil)
