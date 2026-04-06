package memory

import (
	"context"
	"errors"
	"time"
)

type RedisClient interface {
	LRange(ctx context.Context, key string, start int64, stop int64) ([]string, error)
	Del(ctx context.Context, key string) error
	Unlink(ctx context.Context, key string) error
	RPush(ctx context.Context, key string, values []string) error
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type CacheStore struct {
	client RedisClient
}

func NewCacheStore(client RedisClient) (*CacheStore, error) {
	if client == nil {
		return nil, errors.New("nil redis client")
	}
	return &CacheStore{client: client}, nil
}

func (c *CacheStore) Range(ctx context.Context, key string, start int64, stop int64) ([]string, error) {
	return c.client.LRange(ctx, key, start, stop)
}

func (c *CacheStore) ReplaceWindow(ctx context.Context, key string, payloads []string, ttl time.Duration) error {
	if err := c.Delete(ctx, key); err != nil {
		return err
	}
	if len(payloads) == 0 {
		return nil
	}
	if err := c.client.RPush(ctx, key, payloads); err != nil {
		return err
	}
	return c.client.Expire(ctx, key, ttl)
}

func (c *CacheStore) Delete(ctx context.Context, key string) error {
	if err := c.client.Unlink(ctx, key); err != nil {
		return c.client.Del(ctx, key)
	}
	return nil
}