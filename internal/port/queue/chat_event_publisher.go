package queue

import "context"

type ChatEventPublisher interface {
	PublishChatEvent(ctx context.Context, agentID string, sessionID string, userInput string) error
}