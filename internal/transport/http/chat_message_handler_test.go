package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appchat "go-sse-skeleton/internal/app/chat"
	domain "go-sse-skeleton/internal/domain/chat"
)

type fakeChatService struct {
	createFn     func(ctx context.Context, cmd appchat.CreateMessageCommand) (string, error)
	appendFn     func(ctx context.Context, id string, appendContent string) error
	listRecentFn func(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error)
}

func (f *fakeChatService) CreateFromCommand(ctx context.Context, cmd appchat.CreateMessageCommand) (string, error) {
	return f.createFn(ctx, cmd)
}
func (f *fakeChatService) CreateInternal(context.Context, *domain.Message) (string, error) {
	return "", nil
}
func (f *fakeChatService) Append(ctx context.Context, messageID string, appendContent string) error {
	return f.appendFn(ctx, messageID, appendContent)
}
func (f *fakeChatService) ListBySession(context.Context, string) ([]*domain.Message, error) { return nil, nil }
func (f *fakeChatService) ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error) {
	return f.listRecentFn(ctx, sessionID, limit)
}
func (f *fakeChatService) Update(context.Context, string, appchat.UpdateMessageCommand) error { return nil }
func (f *fakeChatService) Delete(context.Context, string) error                                { return nil }

func TestCreateResponseUsesApiResponseShape(t *testing.T) {
	t.Parallel()
	h, err := NewChatMessageHandler(&fakeChatService{
		createFn: func(_ context.Context, _ appchat.CreateMessageCommand) (string, error) { return "m1", nil },
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := `{"agentId":"a1","sessionId":"s1","role":"user","content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat-messages", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var resp map[string]any
	if err = json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if int(resp["code"].(float64)) != 200 {
		t.Fatalf("unexpected code field: %+v", resp)
	}
	if resp["message"] != "success" {
		t.Fatalf("unexpected message field: %+v", resp)
	}
	data := resp["data"].(map[string]any)
	if data["chatMessageId"] != "m1" {
		t.Fatalf("unexpected chatMessageId: %+v", data)
	}
}

func TestAppendNotFoundMapsTo404(t *testing.T) {
	t.Parallel()
	h, err := NewChatMessageHandler(&fakeChatService{
		appendFn: func(_ context.Context, _ string, _ string) error {
			return &appchat.AppError{Code: appchat.ErrorCodeNotFound, Err: domain.ErrNotFound}
		},
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/chat-messages/missing/append", bytes.NewBufferString(`{"appendContent":"x"}`))
	rec := httptest.NewRecorder()

	h.Append(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var resp map[string]any
	if err = json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["errorCode"] != string(appchat.ErrorCodeNotFound) {
		t.Fatalf("unexpected errorCode: %+v", resp)
	}
}

func TestListRecentSupportsMetadataFields(t *testing.T) {
	t.Parallel()
	h, err := NewChatMessageHandler(&fakeChatService{
		listRecentFn: func(_ context.Context, _ string, _ int) ([]*domain.Message, error) {
			return []*domain.Message{
				{
					ID:        "m1",
					SessionID: "s1",
					Role:      domain.RoleAssistant,
					Content:   "c1",
					Metadata: &domain.Metadata{
						ToolCalls: []any{"call1"},
					},
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chat-messages/recent?sessionId=s1&limit=1", nil)
	rec := httptest.NewRecorder()
	h.ListRecentBySession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp map[string]any
	if err = json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]any)
	chatMessages := data["chatMessages"].([]any)
	if len(chatMessages) != 1 {
		t.Fatalf("unexpected chatMessages len: %d", len(chatMessages))
	}
	msg := chatMessages[0].(map[string]any)
	metadata := msg["Metadata"].(map[string]any)
	toolCalls := metadata["toolCalls"].([]any)
	if len(toolCalls) != 1 || toolCalls[0] != "call1" {
		t.Fatalf("unexpected metadata toolCalls: %+v", metadata)
	}
}
