package inmemory

import "context"

type ChatEventPublisher struct{}

func NewChatEventPublisher() *ChatEventPublisher {
	return &ChatEventPublisher{}
}

func (p *ChatEventPublisher) PublishChatEvent(ctx context.Context, agentID string, sessionID string, userInput string) error {
	_ = ctx
	_ = agentID
	_ = sessionID
	_ = userInput
	// TODO: publish to async bus / queue implementation.
	return nil
}