package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	app "go-sse-skeleton/internal/app/sse"
)

type SSEHandler struct {
	notifier app.Notifier
	hub      interface {
		Connect(ctx context.Context, sessionID string, w http.ResponseWriter, r *http.Request) error
	}
}

func NewSSEHandler(notifier app.Notifier, hub interface {
	Connect(ctx context.Context, sessionID string, w http.ResponseWriter, r *http.Request) error
}) (*SSEHandler, error) {
	if notifier == nil || hub == nil {
		return nil, errors.New("nil dependency in sse handler")
	}
	return &SSEHandler{notifier: notifier, hub: hub}, nil
}

func (h *SSEHandler) Connect(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/sse/connect/")
	if sessionID == "" {
		http.Error(w, "missing chatSessionId", http.StatusBadRequest)
		return
	}

	if err := h.hub.Connect(r.Context(), sessionID, w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}