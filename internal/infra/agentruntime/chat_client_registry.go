package agentruntime

import (
	"context"
	"errors"

	port "go-sse-skeleton/internal/port/agentruntime"
	"go-sse-skeleton/internal/port/llm"
)

type ChatClientRegistry struct {
	byModel map[string]port.ChatClient
}

func NewChatClientRegistry(byModel map[string]port.ChatClient) *ChatClientRegistry {
	copied := make(map[string]port.ChatClient, len(byModel))
	for k, v := range byModel {
		copied[k] = v
	}
	return &ChatClientRegistry{byModel: copied}
}

func NewSingleClientRegistry(model string, client llm.Client) (*ChatClientRegistry, error) {
	if model == "" || client == nil {
		return nil, errors.New("invalid single client registry input")
	}
	return NewChatClientRegistry(map[string]port.ChatClient{model: client}), nil
}

func (r *ChatClientRegistry) GetByModel(_ context.Context, model string) (port.ChatClient, error) {
	if model == "" {
		return nil, errors.New("empty model")
	}
	client, ok := r.byModel[model]
	if !ok || client == nil {
		return nil, errors.New("chat client not found")
	}
	return client, nil
}

