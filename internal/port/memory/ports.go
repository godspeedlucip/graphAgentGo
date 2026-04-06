package memory

import (
	"context"
	"time"

	domain "go-sse-skeleton/internal/domain/memory"
)

type CacheStore interface {
	Range(ctx context.Context, key string, start int64, stop int64) ([]string, error)
	ReplaceWindow(ctx context.Context, key string, payloads []string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type MessageReader interface {
	ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]domain.Message, error)
}

type Codec interface {
	EncodeCached(messages []domain.CachedMessage) ([]string, error)
	DecodeCached(payloads []string) ([]domain.CachedMessage, error)
	RuntimeToCached(messages []domain.Message) ([]domain.CachedMessage, error)
	CachedToRuntime(messages []domain.CachedMessage) ([]domain.Message, error)
}

type MQPublisher interface {
	Publish(ctx context.Context, topic string, payload any) error
}

type WebsocketPusher interface {
	Push(ctx context.Context, channel string, payload any) error
}

type PaymentGateway interface {
	Charge(ctx context.Context, userID string, amount int64, currency string) (string, error)
}

// TxManager is intentionally defined at app-port boundary.
// Transaction orchestration belongs to app/usecase layer.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}