package event

import (
	"context"
	"time"

	domain "go-sse-skeleton/internal/domain/event"
)

type Publisher interface {
	PublishChatEvent(ctx context.Context, evt domain.ChatEvent) error
}

type Subscriber interface {
	SubscribeChatEvent(ctx context.Context, handler ChatEventHandler) error
}

type ChatEventHandler interface {
	Handle(ctx context.Context, evt domain.ChatEvent) error
}

type Dispatcher interface {
	Submit(ctx context.Context, job func(context.Context) error) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// DedupStore provides atomic idempotency mark for at-least-once delivery.
type DedupStore interface {
	MarkIfAbsent(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Observer is a minimal extensibility point for metrics/tracing sinks.
type Observer interface {
	RecordPublished(topic string)
	RecordConsumed(topic string, latency time.Duration)
	RecordFailed(topic string, stage string)
	RecordQueueLen(topic string, length int)
}

// DeadLetterSink stores events that exhausted retries.
type DeadLetterSink interface {
	Push(ctx context.Context, evt domain.ChatEvent, reason string, attempts int) error
}

type BrokerMessage struct {
	Topic   string
	Key     string
	Payload []byte
}

// MessageBroker is an adapter port for real MQ implementations.
type MessageBroker interface {
	Publish(ctx context.Context, msg BrokerMessage) error
	Subscribe(ctx context.Context, topic string, handler func(context.Context, BrokerMessage) error) error
}
