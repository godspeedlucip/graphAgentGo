package bootstrap

import (
	"fmt"

	appsse "go-sse-skeleton/internal/app/sse"
	infrasse "go-sse-skeleton/internal/infra/sse"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	"go-sse-skeleton/internal/port/repository"
	transport "go-sse-skeleton/internal/transport/http"
)

type App struct {
	SSEService *appsse.Service
	SSEHandler *transport.SSEHandler
}

func NewApp(repo repository.ChatMessageRepository, q queue.EventPublisher, l llm.Client) (*App, error) {
	hub := infrasse.NewHub()

	svc, err := appsse.NewService(repo, q, hub, l)
	if err != nil {
		return nil, fmt.Errorf("new sse service: %w", err)
	}

	handler, err := transport.NewSSEHandler(svc, hub)
	if err != nil {
		return nil, fmt.Errorf("new sse handler: %w", err)
	}

	return &App{SSEService: svc, SSEHandler: handler}, nil
}