package memory

import (
	"context"
	"encoding/json"
	"errors"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	memorydomain "go-sse-skeleton/internal/domain/memory"
	repo "go-sse-skeleton/internal/port/repository"
)

type ChatStoreMessageReader struct {
	store repo.ChatMessageStore
}

func NewChatStoreMessageReader(store repo.ChatMessageStore) (*ChatStoreMessageReader, error) {
	if store == nil {
		return nil, errors.New("nil chat message store")
	}
	return &ChatStoreMessageReader{store: store}, nil
}

func (r *ChatStoreMessageReader) ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]memorydomain.Message, error) {
	rows, err := r.store.ListRecentBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]memorydomain.Message, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, memorydomain.Message{
			Role:     memorydomain.Role(row.Role),
			Content:  row.Content,
			Metadata: convertMetadata(row.Metadata),
		})
	}
	return out, nil
}

func convertMetadata(m *chatdomain.Metadata) *memorydomain.Metadata {
	if m == nil {
		return nil
	}
	out := &memorydomain.Metadata{
		ToolCalls: make([]memorydomain.ToolCall, 0, len(m.ToolCalls)),
	}
	for _, tc := range m.ToolCalls {
		parsed, ok := toToolCall(tc)
		if ok {
			out.ToolCalls = append(out.ToolCalls, parsed)
		}
	}
	if tr, ok := toToolResponse(m.ToolResponse); ok {
		out.ToolResponse = &tr
	}
	if len(out.ToolCalls) == 0 && out.ToolResponse == nil {
		return nil
	}
	return out
}

func toToolCall(v any) (memorydomain.ToolCall, bool) {
	if v == nil {
		return memorydomain.ToolCall{}, false
	}
	if tc, ok := v.(memorydomain.ToolCall); ok {
		return tc, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return memorydomain.ToolCall{}, false
	}
	var tc memorydomain.ToolCall
	if err = json.Unmarshal(b, &tc); err != nil {
		return memorydomain.ToolCall{}, false
	}
	if tc.ID == "" && tc.Type == "" && tc.Name == "" && tc.Arguments == "" {
		return memorydomain.ToolCall{}, false
	}
	return tc, true
}

func toToolResponse(v any) (memorydomain.ToolResponse, bool) {
	if v == nil {
		return memorydomain.ToolResponse{}, false
	}
	if tr, ok := v.(memorydomain.ToolResponse); ok {
		return tr, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return memorydomain.ToolResponse{}, false
	}
	var tr memorydomain.ToolResponse
	if err = json.Unmarshal(b, &tr); err != nil {
		return memorydomain.ToolResponse{}, false
	}
	if tr.ID == "" && tr.Name == "" && tr.ResponseData == "" {
		return memorydomain.ToolResponse{}, false
	}
	return tr, true
}
