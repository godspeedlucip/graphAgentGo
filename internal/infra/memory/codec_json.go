package memory

import (
	"encoding/json"
	"strings"

	domain "go-sse-skeleton/internal/domain/memory"
)

type JSONCodec struct{}

func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

func (c *JSONCodec) EncodeCached(messages []domain.CachedMessage) ([]string, error) {
	out := make([]string, 0, len(messages))
	for _, msg := range messages {
		b, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		out = append(out, string(b))
	}
	return out, nil
}

func (c *JSONCodec) DecodeCached(payloads []string) ([]domain.CachedMessage, error) {
	out := make([]domain.CachedMessage, 0, len(payloads))
	for _, payload := range payloads {
		var msg domain.CachedMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}

func (c *JSONCodec) RuntimeToCached(messages []domain.Message) ([]domain.CachedMessage, error) {
	out := make([]domain.CachedMessage, 0, len(messages))
	for _, msg := range messages {
		if !domain.IsValidRole(msg.Role) {
			continue
		}
		out = append(out, domain.CachedMessage{Role: msg.Role, Content: msg.Content, Metadata: msg.Metadata})
	}
	return out, nil
}

func (c *JSONCodec) CachedToRuntime(messages []domain.CachedMessage) ([]domain.Message, error) {
	out := make([]domain.Message, 0, len(messages))
	canAppendToolResponse := false

	for _, msg := range messages {
		if !domain.IsValidRole(msg.Role) {
			continue
		}

		switch msg.Role {
		case domain.RoleSystem, domain.RoleUser:
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			out = append(out, domain.Message{Role: msg.Role, Content: msg.Content, Metadata: msg.Metadata})
			canAppendToolResponse = false
		case domain.RoleAssistant:
			hasText := strings.TrimSpace(msg.Content) != ""
			hasToolCalls := domain.HasToolCalls(msg.Metadata)
			if !hasText && !hasToolCalls {
				continue
			}
			out = append(out, domain.Message{Role: msg.Role, Content: msg.Content, Metadata: msg.Metadata})
			canAppendToolResponse = hasToolCalls
		case domain.RoleTool:
			// Keep Java behavior:
			// skip orphan tool-response messages that are not preceded by assistant tool-calls.
			if !canAppendToolResponse || !domain.HasToolResponse(msg.Metadata) {
				continue
			}
			out = append(out, domain.Message{Role: msg.Role, Content: msg.Content, Metadata: msg.Metadata})
		}
	}
	return out, nil
}
