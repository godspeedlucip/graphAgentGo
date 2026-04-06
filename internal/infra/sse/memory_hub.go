package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	domain "go-sse-skeleton/internal/domain/sse"
)

const (
	connectTimeout = 30 * time.Minute // align with Java SseEmitter(30 * 60 * 1000L)
)

type clientConn struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex // serialize writes per connection
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*clientConn
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]*clientConn)}
}

func (h *Hub) Connect(ctx context.Context, sessionID string, w http.ResponseWriter, _ *http.Request) error {
	if sessionID == "" {
		return domain.ErrInvalidInput
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	conn := &clientConn{w: w, flusher: flusher}

	h.mu.Lock()
	h.clients[sessionID] = conn
	h.mu.Unlock()

	if err := writeEvent(conn, "init", []byte("connected")); err != nil {
		_ = h.Disconnect(ctx, sessionID)
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	// TODO: add heartbeat ping event for proxies/load-balancers that close idle SSE connections.
	<-waitCtx.Done()
	return h.Disconnect(context.Background(), sessionID)
}

func (h *Hub) Publish(ctx context.Context, sessionID string, msg domain.Message) error {
	if sessionID == "" {
		return domain.ErrInvalidInput
	}
	if err := msg.Validate(); err != nil {
		return err
	}

	conn, err := h.getConn(sessionID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err = writeEvent(conn, "message", payload); err != nil {
		_ = h.Disconnect(ctx, sessionID)
		return domain.ErrClientDisconnected
	}
	return nil
}

func (h *Hub) Disconnect(_ context.Context, sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, sessionID)
	return nil
}

func (h *Hub) getConn(sessionID string) (*clientConn, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	conn, ok := h.clients[sessionID]
	if !ok || conn == nil {
		return nil, domain.ErrClientNotFound
	}
	return conn, nil
}

func writeEvent(conn *clientConn, event string, data []byte) error {
	if conn == nil {
		return domain.ErrClientNotFound
	}
	if event == "" {
		return domain.ErrInvalidInput
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if _, err := fmt.Fprintf(conn.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	conn.flusher.Flush()

	return nil
}

func IsClientGone(err error) bool {
	return errors.Is(err, domain.ErrClientNotFound) || errors.Is(err, domain.ErrClientDisconnected)
}