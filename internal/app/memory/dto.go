package memory

import (
	domain "go-sse-skeleton/internal/domain/memory"
)

type Config struct {
	MaxMessages int
	TTLHours    int
}

type GetResult struct {
	Messages []domain.Message
	Source   string // cache | db_fallback
}