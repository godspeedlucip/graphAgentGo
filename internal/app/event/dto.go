package event

import domain "go-sse-skeleton/internal/domain/event"

type PublishChatEventCommand struct {
	Event domain.ChatEvent
}