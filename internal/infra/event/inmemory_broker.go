package event

import (
	"context"
	"sync"

	port "go-sse-skeleton/internal/port/event"
)

type InMemoryBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]func(context.Context, port.BrokerMessage) error
}

func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{
		subscribers: map[string][]func(context.Context, port.BrokerMessage) error{},
	}
}

func (b *InMemoryBroker) Publish(ctx context.Context, msg port.BrokerMessage) error {
	b.mu.RLock()
	handlers := append([]func(context.Context, port.BrokerMessage) error(nil), b.subscribers[msg.Topic]...)
	b.mu.RUnlock()
	for _, h := range handlers {
		if err := h(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (b *InMemoryBroker) Subscribe(_ context.Context, topic string, handler func(context.Context, port.BrokerMessage) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[topic] = append(b.subscribers[topic], handler)
	return nil
}

var _ port.MessageBroker = (*InMemoryBroker)(nil)
