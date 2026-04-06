package redis

import (
	"context"
	"errors"
)

type Client interface {
	Unlink(ctx context.Context, key string) error
	Del(ctx context.Context, key string) error
}

type ChatMemoryCache struct {
	client Client
}

func NewChatMemoryCache(client Client) (*ChatMemoryCache, error) {
	if client == nil {
		return nil, errors.New("nil redis client")
	}
	return &ChatMemoryCache{client: client}, nil
}

func (c *ChatMemoryCache) Invalidate(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("invalid sessionID")
	}
	key := "chat:memory:" + sessionID
	if err := c.client.Unlink(ctx, key); err != nil {
		// fallback behavior aligns with Java: unlink fail -> del
		return c.client.Del(ctx, key)
	}
	return nil
}