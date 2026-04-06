package repository

import (
	"context"

	domain "go-sse-skeleton/internal/domain/chat"
)

type ChatMessageStore interface {
	Create(ctx context.Context, msg *domain.Message) (string, error)
	GetByID(ctx context.Context, id string) (*domain.Message, error)
	ListBySession(ctx context.Context, sessionID string) ([]*domain.Message, error)
	ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error)
	Update(ctx context.Context, msg *domain.Message) error
	Delete(ctx context.Context, id string) error
}