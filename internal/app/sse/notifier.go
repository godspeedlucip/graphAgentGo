package sse

import (
	"context"

	domain "go-sse-skeleton/internal/domain/sse"
)

type Notifier interface {
	Send(ctx context.Context, sessionID string, msg domain.Message) error
	SendGeneratedContent(ctx context.Context, sessionID string, chatMessageID string, message any) error
	StartAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error
	AppendAssistantDelta(ctx context.Context, sessionID string, chatMessageID string, delta string) error
	EndAssistantStream(ctx context.Context, sessionID string, chatMessageID string) error
	NotifyDone(ctx context.Context, sessionID string) error
}