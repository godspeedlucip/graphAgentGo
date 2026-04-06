package sse

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/sse"
)

func TestHubConnectSendsHeartbeatPing(t *testing.T) {
	t.Parallel()

	hub := NewHub(
		WithHeartbeatInterval(20*time.Millisecond),
		WithConnectTimeout(2*time.Second),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.Connect(r.Context(), "session-heartbeat", w, r)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	initEvent, err := readEvent(reader)
	if err != nil {
		t.Fatalf("read init event: %v", err)
	}
	if initEvent.event != "init" {
		t.Fatalf("expected init event, got %q", initEvent.event)
	}
	if initEvent.data != "connected" {
		t.Fatalf("expected init data connected, got %q", initEvent.data)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		ev, readErr := readEvent(reader)
		if readErr != nil {
			t.Fatalf("read ping event: %v", readErr)
		}
		if ev.event == "ping" && ev.data == "keepalive" {
			return
		}
	}
	t.Fatal("expected ping heartbeat event but none received")
}

func TestHubDisconnectCleanupAfterClientClose(t *testing.T) {
	t.Parallel()

	hub := NewHub(
		WithHeartbeatInterval(10*time.Millisecond),
		WithConnectTimeout(5*time.Second),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.Connect(r.Context(), "session-cleanup", w, r)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}

	// Consume init once then close client side.
	reader := bufio.NewReader(resp.Body)
	if _, err = readEvent(reader); err != nil {
		t.Fatalf("read init event: %v", err)
	}
	_ = resp.Body.Close()

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		err = hub.Publish(context.Background(), "session-cleanup", domain.Message{Type: domain.TypeAIThinking})
		if err == nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if err == domain.ErrClientNotFound || err == domain.ErrClientDisconnected {
			return
		}
		t.Fatalf("unexpected publish error after close: %v", err)
	}
	t.Fatal("expected client cleanup after close, but publish kept succeeding")
}

type eventFrame struct {
	event string
	data  string
}

func readEvent(reader *bufio.Reader) (eventFrame, error) {
	var frame eventFrame
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return eventFrame{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if frame.event != "" || frame.data != "" {
				return frame, nil
			}
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			frame.event = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			frame.data = strings.TrimPrefix(line, "data: ")
		}
	}
}
