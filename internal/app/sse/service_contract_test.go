package sse_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appsse "go-sse-skeleton/internal/app/sse"
	domainsse "go-sse-skeleton/internal/domain/sse"
	infrasse "go-sse-skeleton/internal/infra/sse"
	transport "go-sse-skeleton/internal/transport/http"
)

type fakeRepo struct {
	mu             sync.Mutex
	createCalls    int
	lastSessionID  string
	lastRole       string
	lastContent    string
	lastGenerated  string
}

func (r *fakeRepo) Create(_ context.Context, sessionID string, role string, content string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createCalls++
	r.lastSessionID = sessionID
	r.lastRole = role
	r.lastContent = content
	r.lastGenerated = "assistant-msg-1"
	return r.lastGenerated, nil
}

func (r *fakeRepo) AppendContent(_ context.Context, _ string, _ string) error { return nil }

type noopQueue struct{}

func (noopQueue) Publish(context.Context, string, any) error { return nil }

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

type sseEnvelope struct {
	Type     string `json:"type"`
	Payload  struct {
		Message      map[string]any `json:"message"`
		DeltaContent string         `json:"deltaContent"`
		Done         *bool          `json:"done"`
	} `json:"payload"`
	Metadata struct {
		ChatMessageID string `json:"chatMessageId"`
	} `json:"metadata"`
}

func TestSSEContractInitStartDeltaEndDone(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	hub := infrasse.NewHub(
		infrasse.WithHeartbeatInterval(0), // keep deterministic event order in this contract test
		infrasse.WithConnectTimeout(5*time.Second),
	)
	svc, err := appsse.NewService(repo, noopQueue{}, hub, noopLLM{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	handler, err := transport.NewSSEHandler(svc, hub)
	if err != nil {
		t.Fatalf("new sse handler: %v", err)
	}

	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/sse/connect/session-1")
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	initEvent, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read init event: %v", err)
	}
	if initEvent.Event != "init" {
		t.Fatalf("expected init event, got %q", initEvent.Event)
	}
	if initEvent.Data != "connected" {
		t.Fatalf("expected init data connected, got %q", initEvent.Data)
	}

	if err = svc.StartAssistantStream(context.Background(), "session-1", ""); err != nil {
		t.Fatalf("start assistant stream: %v", err)
	}

	startEvent, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read start event: %v", err)
	}
	if startEvent.Event != "message" {
		t.Fatalf("expected message event for start, got %q", startEvent.Event)
	}

	var startMsg sseEnvelope
	if err = json.Unmarshal([]byte(startEvent.Data), &startMsg); err != nil {
		t.Fatalf("unmarshal start json: %v", err)
	}
	if startMsg.Type != string(domainsse.TypeAIGeneratedContentStart) {
		t.Fatalf("unexpected start type: %s", startMsg.Type)
	}
	if startMsg.Metadata.ChatMessageID != "assistant-msg-1" {
		t.Fatalf("unexpected start chatMessageId: %s", startMsg.Metadata.ChatMessageID)
	}
	if startMsg.Payload.Message["id"] != "assistant-msg-1" {
		t.Fatalf("start payload.message.id mismatch: %+v", startMsg.Payload.Message)
	}
	if startMsg.Payload.Message["sessionId"] != "session-1" {
		t.Fatalf("start payload.message.sessionId mismatch: %+v", startMsg.Payload.Message)
	}
	if startMsg.Payload.Message["role"] != "assistant" {
		t.Fatalf("start payload.message.role mismatch: %+v", startMsg.Payload.Message)
	}

	if err = svc.AppendAssistantDelta(context.Background(), "session-1", "assistant-msg-1", "Hello"); err != nil {
		t.Fatalf("append assistant delta: %v", err)
	}
	if err = svc.EndAssistantStream(context.Background(), "session-1", "assistant-msg-1"); err != nil {
		t.Fatalf("end assistant stream: %v", err)
	}
	if err = svc.NotifyDone(context.Background(), "session-1"); err != nil {
		t.Fatalf("notify done: %v", err)
	}

	deltaEvent, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read delta event: %v", err)
	}
	var deltaMsg sseEnvelope
	if err = json.Unmarshal([]byte(deltaEvent.Data), &deltaMsg); err != nil {
		t.Fatalf("unmarshal delta json: %v", err)
	}
	if deltaMsg.Type != string(domainsse.TypeAIGeneratedContentDelta) {
		t.Fatalf("unexpected delta type: %s", deltaMsg.Type)
	}
	if deltaMsg.Payload.DeltaContent != "Hello" {
		t.Fatalf("unexpected delta content: %q", deltaMsg.Payload.DeltaContent)
	}
	if deltaMsg.Metadata.ChatMessageID != "assistant-msg-1" {
		t.Fatalf("unexpected delta chatMessageId: %s", deltaMsg.Metadata.ChatMessageID)
	}

	endEvent, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read end event: %v", err)
	}
	var endMsg sseEnvelope
	if err = json.Unmarshal([]byte(endEvent.Data), &endMsg); err != nil {
		t.Fatalf("unmarshal end json: %v", err)
	}
	if endMsg.Type != string(domainsse.TypeAIGeneratedContentEnd) {
		t.Fatalf("unexpected end type: %s", endMsg.Type)
	}
	if endMsg.Payload.Done == nil || !*endMsg.Payload.Done {
		t.Fatalf("expected end done=true, got %+v", endMsg.Payload.Done)
	}
	if endMsg.Metadata.ChatMessageID != "assistant-msg-1" {
		t.Fatalf("unexpected end chatMessageId: %s", endMsg.Metadata.ChatMessageID)
	}

	doneEvent, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read done event: %v", err)
	}
	var doneMsg sseEnvelope
	if err = json.Unmarshal([]byte(doneEvent.Data), &doneMsg); err != nil {
		t.Fatalf("unmarshal done json: %v", err)
	}
	if doneMsg.Type != string(domainsse.TypeAIDone) {
		t.Fatalf("unexpected done type: %s", doneMsg.Type)
	}
	if doneMsg.Payload.Done == nil || !*doneMsg.Payload.Done {
		t.Fatalf("expected done done=true, got %+v", doneMsg.Payload.Done)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.createCalls != 1 {
		t.Fatalf("expected exactly one Create call, got %d", repo.createCalls)
	}
	if repo.lastSessionID != "session-1" || repo.lastRole != "assistant" || repo.lastContent != "" {
		t.Fatalf("unexpected Create args: session=%s role=%s content=%q", repo.lastSessionID, repo.lastRole, repo.lastContent)
	}
}

type parsedSSEEvent struct {
	Event string
	Data  string
}

func readSSEEvent(reader *bufio.Reader) (parsedSSEEvent, error) {
	var event parsedSSEEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return parsedSSEEvent{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if event.Event != "" || event.Data != "" {
				return event, nil
			}
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			event.Event = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			dataLine := strings.TrimPrefix(line, "data: ")
			if event.Data == "" {
				event.Data = dataLine
			} else {
				event.Data += "\n" + dataLine
			}
		}
	}
}
