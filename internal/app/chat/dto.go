package chat

import (
	domain "go-sse-skeleton/internal/domain/chat"
)

type CreateMessageCommand struct {
	AgentID   string
	SessionID string
	Role      domain.Role
	Content   string
	Metadata  *domain.Metadata
}

type UpdateMessageCommand struct {
	Content  *string
	Metadata *domain.Metadata
}