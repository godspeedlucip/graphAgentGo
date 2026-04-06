package event

import (
	"context"
	"errors"
	"log/slog"
	"time"

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
	dedup        port.DedupStore
	dedupTTL     time.Duration
	observer     port.Observer
}

func NewConsumerService(
	subscriber port.Subscriber,
	dispatcher port.Dispatcher,
	runner AgentRunner,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
	opts ...Option,
) (ConsumerService, error) {
	if subscriber == nil || dispatcher == nil || runner == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in event consumer service")
	}
	svc := &consumerService{
		subscriber:   subscriber,
		dispatcher:   dispatcher,
		runner:       runner,
		messageStore: messageStore,
		eventBus:     eventBus,
		sseNotifier:  sseNotifier,
		llmClient:    llmClient,
		dedup:        noopDedupStore{},
		dedupTTL:     24 * time.Hour,
		observer:     noopObserver{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc, nil
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
		s.observer.RecordFailed("chat_event", "validate")
		return err
	}
	if evt.EventID != "" {
		key := "chat_event:" + evt.EventID
		accepted, err := s.dedup.MarkIfAbsent(ctx, key, s.dedupTTL)
		if err != nil {
			s.observer.RecordFailed("chat_event", "dedup")
			return err
		}
		if !accepted {
			// At-least-once delivery can send duplicates; idempotency store suppresses re-consume.
			return nil
		}
	}

	start := time.Now()
	submitter := s.dispatcher.Submit
	type eventSubmitter interface {
		SubmitEvent(ctx context.Context, evt domain.ChatEvent, job func(context.Context) error) error
	}
	if withEvent, ok := s.dispatcher.(eventSubmitter); ok {
		submitter = func(submitCtx context.Context, job func(context.Context) error) error {
			return withEvent.SubmitEvent(submitCtx, evt, job)
		}
	}
	// Async equivalence of @Async/@EventListener:
	// submit job to dispatcher and return immediately without waiting for runner completion.
	err := submitter(ctx, func(jobCtx context.Context) error {
		if runErr := s.runner.RunByEvent(jobCtx, evt); runErr != nil {
			s.observer.RecordFailed("chat_event", "runner")
			slog.Error("async event handling failed", "eventID", evt.EventID, "agentID", evt.AgentID, "sessionID", evt.SessionID, "err", runErr)
			return runErr
		}
		s.observer.RecordConsumed("chat_event", time.Since(start))
		return nil
	})
	if err != nil {
		s.observer.RecordFailed("chat_event", "submit")
	}
	return err
}

var _ port.ChatEventHandler = (*consumerService)(nil)
