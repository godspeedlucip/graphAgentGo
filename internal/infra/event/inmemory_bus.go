package event

import (
	"context"
	"log/slog"
	"sync"

	domain "go-sse-skeleton/internal/domain/event"
	port "go-sse-skeleton/internal/port/event"
)

type InMemoryBus struct {
	mu       sync.RWMutex
	handlers []port.ChatEventHandler
}

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{}
}

func (b *InMemoryBus) PublishChatEvent(ctx context.Context, evt domain.ChatEvent) error {
	if err := evt.Validate(); err != nil {
		return err
	}

	b.mu.RLock()
	handlers := append([]port.ChatEventHandler(nil), b.handlers...)
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h.Handle(ctx, evt); err != nil {
			// Keep publish path resilient: one consumer failure should not fail event publish.
			slog.Error("inmemory event handler failed", "eventID", evt.EventID, "agentID", evt.AgentID, "sessionID", evt.SessionID, "err", err)
		}
	}
	return nil
}

func (b *InMemoryBus) SubscribeChatEvent(_ context.Context, handler port.ChatEventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
	return nil
}

var _ port.Publisher = (*InMemoryBus)(nil)
var _ port.Subscriber = (*InMemoryBus)(nil)
