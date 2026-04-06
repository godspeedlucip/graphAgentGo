package agentruntime

import (
	"context"
	"errors"

	port "go-sse-skeleton/internal/port/agentruntime"
)

type runtimeMemory struct {
	sessionID   string
	maxMessages int
}

type PlaceholderMemoryBuilder struct{}

func NewPlaceholderMemoryBuilder() *PlaceholderMemoryBuilder {
	return &PlaceholderMemoryBuilder{}
}

func (b *PlaceholderMemoryBuilder) Build(_ context.Context, sessionID string, maxMessages int) (port.Memory, error) {
	if sessionID == "" || maxMessages <= 0 {
		return nil, errors.New("invalid memory build input")
	}
	// TODO: bind to memory service (Redis+DB fallback) once unified memory runtime interface is finalized.
	return &runtimeMemory{sessionID: sessionID, maxMessages: maxMessages}, nil
}
