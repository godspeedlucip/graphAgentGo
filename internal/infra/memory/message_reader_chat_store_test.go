package memory

import (
	"context"
	"testing"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	memorydomain "go-sse-skeleton/internal/domain/memory"
)

type fakeChatStore struct {
	rows []*chatdomain.Message
}

func (s *fakeChatStore) Create(context.Context, *chatdomain.Message) (string, error) {
	return "", nil
}

func (s *fakeChatStore) GetByID(context.Context, string) (*chatdomain.Message, error) {
	return nil, nil
}

func (s *fakeChatStore) ListBySession(context.Context, string) ([]*chatdomain.Message, error) {
	return nil, nil
}

func (s *fakeChatStore) ListRecentBySession(context.Context, string, int) ([]*chatdomain.Message, error) {
	return s.rows, nil
}

func (s *fakeChatStore) Update(context.Context, *chatdomain.Message) error { return nil }

func (s *fakeChatStore) Delete(context.Context, string) error { return nil }

func TestChatStoreMessageReaderConvertToolMetadata(t *testing.T) {
	t.Parallel()

	store := &fakeChatStore{
		rows: []*chatdomain.Message{
			{
				Role:    chatdomain.RoleAssistant,
				Content: "assistant with tool call",
				Metadata: &chatdomain.Metadata{
					ToolCalls: []any{
						map[string]any{
							"id":        "call-1",
							"type":      "function",
							"name":      "weather",
							"arguments": "{\"city\":\"shanghai\"}",
						},
					},
				},
			},
			{
				Role:    chatdomain.RoleTool,
				Content: "tool response",
				Metadata: &chatdomain.Metadata{
					ToolResponse: map[string]any{
						"id":           "resp-1",
						"name":         "weather",
						"responseData": "sunny",
					},
				},
			},
		},
	}

	reader, err := NewChatStoreMessageReader(store)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	out, err := reader.ListRecentBySession(context.Background(), "s1", 20)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("unexpected result size: %d", len(out))
	}
	if len(out[0].Metadata.ToolCalls) != 1 {
		t.Fatalf("toolCalls conversion failed: %+v", out[0].Metadata)
	}
	if out[0].Metadata.ToolCalls[0].ID != "call-1" || out[0].Metadata.ToolCalls[0].Name != "weather" {
		t.Fatalf("toolCall fields mismatch: %+v", out[0].Metadata.ToolCalls[0])
	}
	if out[1].Metadata.ToolResponse == nil {
		t.Fatalf("toolResponse conversion failed: %+v", out[1].Metadata)
	}
	if out[1].Metadata.ToolResponse.ID != "resp-1" || out[1].Metadata.ToolResponse.ResponseData != "sunny" {
		t.Fatalf("toolResponse fields mismatch: %+v", out[1].Metadata.ToolResponse)
	}
	if out[0].Role != memorydomain.RoleAssistant || out[1].Role != memorydomain.RoleTool {
		t.Fatalf("role conversion mismatch: %+v", out)
	}
}
