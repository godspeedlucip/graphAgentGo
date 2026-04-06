package sse

import (
	"context"
	"net/http"

	domain "go-sse-skeleton/internal/domain/sse"
)

type Hub interface {
	Connect(ctx context.Context, sessionID string, w http.ResponseWriter, r *http.Request) error
	Publish(ctx context.Context, sessionID string, msg domain.Message) error
	Disconnect(ctx context.Context, sessionID string) error
}