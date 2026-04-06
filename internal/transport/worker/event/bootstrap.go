package event

import (
	"context"
	"errors"

	app "go-sse-skeleton/internal/app/event"
	domain "go-sse-skeleton/internal/domain/event"
)

type Bootstrap struct {
	consumer app.ConsumerService
}

func NewBootstrap(consumer app.ConsumerService) (*Bootstrap, error) {
	if consumer == nil {
		return nil, errors.New("nil event consumer")
	}
	return &Bootstrap{consumer: consumer}, nil
}

func (b *Bootstrap) Start(ctx context.Context) error {
	// Starts async event consumption loop.
	return b.consumer.Start(ctx)
}

func (b *Bootstrap) Stop(ctx context.Context) error {
	// Gracefully stops worker pool and subscription processing.
	return b.consumer.Stop(ctx)
}

// NoopRunner is transport-layer stub; replace with real runner wiring in bootstrap package.
type NoopRunner struct{}

func (NoopRunner) RunByEvent(context.Context, domain.ChatEvent) error {
	return nil
}
