package queue

import "context"

type ChatEventPublisher interface {
	// Adapter point: current default implementation is in-memory noop; production can provide MQ/Kafka/Rabbit publisher.
	PublishChatEvent(ctx context.Context, agentID string, sessionID string, userInput string) error
}
