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
	defaultConnectTimeout   = 30 * time.Minute // align with Java SseEmitter(30 * 60 * 1000L)
	defaultHeartbeatInterval = 15 * time.Second
)

type clientConn struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex // serialize writes per connection
}

type Hub struct {
	mu                sync.RWMutex
	clients           map[string]*clientConn
	connectTimeout    time.Duration
	heartbeatInterval time.Duration
	telemetry         HubTelemetry
}

type HubOption func(*Hub)

func WithConnectTimeout(timeout time.Duration) HubOption {
	return func(h *Hub) {
		if timeout > 0 {
			h.connectTimeout = timeout
		}
	}
}

func WithHeartbeatInterval(interval time.Duration) HubOption {
	return func(h *Hub) {
		if interval >= 0 {
			h.heartbeatInterval = interval
		}
	}
}

func WithHubTelemetry(t HubTelemetry) HubOption {
	return func(h *Hub) {
		if t != nil {
			h.telemetry = t
		}
	}
}

func NewHub(opts ...HubOption) *Hub {
	h := &Hub{
		clients:           make(map[string]*clientConn),
		connectTimeout:    defaultConnectTimeout,
		heartbeatInterval: defaultHeartbeatInterval,
		telemetry:         NewNoopHubTelemetry(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	if h.telemetry == nil {
		h.telemetry = NewNoopHubTelemetry()
	}
	return h
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
	h.telemetry.RecordConnect(sessionID)

	if err := writeEvent(conn, "init", []byte("connected")); err != nil {
		h.telemetry.RecordWrite(sessionID, "init", classifyHubErr(err))
		_ = h.disconnect(sessionID, "init_failed")
		return err
	}
	h.telemetry.RecordWrite(sessionID, "init", "ok")

	waitCtx, cancel := context.WithTimeout(ctx, h.connectTimeout)
	defer cancel()

	if h.heartbeatInterval <= 0 {
		<-waitCtx.Done()
		reason := "timeout"
		if errors.Is(waitCtx.Err(), context.Canceled) {
			reason = "context_canceled"
		}
		return h.disconnect(sessionID, reason)
	}

	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			reason := "timeout"
			if errors.Is(waitCtx.Err(), context.Canceled) {
				reason = "context_canceled"
			}
			return h.disconnect(sessionID, reason)
		case <-ticker.C:
			if err := writeEvent(conn, "ping", []byte("keepalive")); err != nil {
				h.telemetry.RecordHeartbeat(sessionID, false, classifyHubErr(err))
				_ = h.disconnect(sessionID, "heartbeat_failed")
				// Client disconnect during heartbeat is normal and should not be treated as handler failure.
				return nil
			}
			h.telemetry.RecordHeartbeat(sessionID, true, "ok")
		}
	}
}

func (h *Hub) Publish(_ context.Context, sessionID string, msg domain.Message) error {
	if sessionID == "" {
		return domain.ErrInvalidInput
	}
	if err := msg.Validate(); err != nil {
		return err
	}

	conn, err := h.getConn(sessionID)
	if err != nil {
		h.telemetry.RecordWrite(sessionID, "message", classifyHubErr(err))
		return err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		h.telemetry.RecordWrite(sessionID, "message", classifyHubErr(err))
		return err
	}

	if err = writeEvent(conn, "message", payload); err != nil {
		h.telemetry.RecordWrite(sessionID, "message", classifyHubErr(err))
		_ = h.disconnect(sessionID, "publish_failed")
		return domain.ErrClientDisconnected
	}
	h.telemetry.RecordWrite(sessionID, "message", "ok")
	return nil
}

func (h *Hub) Disconnect(_ context.Context, sessionID string) error {
	return h.disconnect(sessionID, "manual")
}

func (h *Hub) disconnect(sessionID string, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[sessionID]; ok {
		delete(h.clients, sessionID)
		h.telemetry.RecordDisconnect(sessionID, reason)
	}
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

type HubTelemetry interface {
	RecordConnect(sessionID string)
	RecordDisconnect(sessionID string, reason string)
	RecordWrite(sessionID string, event string, errClass string)
	RecordHeartbeat(sessionID string, ok bool, errClass string)
}

type NoopHubTelemetry struct{}

func NewNoopHubTelemetry() *NoopHubTelemetry { return &NoopHubTelemetry{} }

func (n *NoopHubTelemetry) RecordConnect(_ string) {}

func (n *NoopHubTelemetry) RecordDisconnect(_ string, _ string) {}

func (n *NoopHubTelemetry) RecordWrite(_ string, _ string, _ string) {}

func (n *NoopHubTelemetry) RecordHeartbeat(_ string, _ bool, _ string) {}

func classifyHubErr(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, domain.ErrClientNotFound):
		return "client_not_found"
	case errors.Is(err, domain.ErrClientDisconnected):
		return "client_disconnected"
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, domain.ErrInvalidMessage):
		return "invalid_message"
	default:
		return "write_failed"
	}
}
