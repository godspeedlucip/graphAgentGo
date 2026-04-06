package repository

import "context"

type ChatMessageRepository interface {
	Create(ctx context.Context, sessionID string, role string, content string) (string, error)
	AppendContent(ctx context.Context, messageID string, delta string) error
}