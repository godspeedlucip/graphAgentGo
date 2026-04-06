package event

import (
	"context"

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