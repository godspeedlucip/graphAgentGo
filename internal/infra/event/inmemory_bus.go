package event

import (
	"context"
	"log/slog"
	"sync"

	domain "go-sse-skeleton/internal/domain/event"
	port "go-sse-skeleton/internal/port/event"
)

type InMemoryBusOption func(*InMemoryBus)

func WithBusObserver(observer port.Observer) InMemoryBusOption {
	return func(b *InMemoryBus) {
		if observer != nil {
			b.observer = observer
		}
	}
}

type InMemoryBus struct {
	mu       sync.RWMutex
	handlers []port.ChatEventHandler
	observer port.Observer
}

func NewInMemoryBus(opts ...InMemoryBusOption) *InMemoryBus {
	b := &InMemoryBus{observer: NewNoopObserver()}
	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}
	return b
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
			b.observer.RecordFailed("chat_event", "handler")
			slog.Error("inmemory event handler failed", "eventID", evt.EventID, "agentID", evt.AgentID, "sessionID", evt.SessionID, "err", err)
		}
	}
	b.observer.RecordPublished("chat_event")
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
