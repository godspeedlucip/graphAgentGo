package cache

import "context"

type ChatMemoryCache interface {
	Invalidate(ctx context.Context, sessionID string) error
}