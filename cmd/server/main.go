package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go-sse-skeleton/internal/bootstrap"
	transport "go-sse-skeleton/internal/transport/http"
)

type noopRepo struct{}

func (noopRepo) Create(context.Context, string, string, string) (string, error) { return "", nil }
func (noopRepo) AppendContent(context.Context, string, string) error             { return nil }

type noopQueue struct{}

func (noopQueue) Publish(context.Context, string, any) error { return nil }

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

func main() {
	app, err := bootstrap.NewApp(noopRepo{}, noopQueue{}, noopLLM{})
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, app.SSEHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}