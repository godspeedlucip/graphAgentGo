package chat

import "time"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type Message struct {
	ID        string
	SessionID string
	Role      Role
	Content   string
	Metadata  *Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Metadata struct {
	ToolResponse any   `json:"toolResponse,omitempty"`
	ToolCalls    []any `json:"toolCalls,omitempty"`
}

func (m *Message) Append(delta string) error {
	if m == nil || delta == "" {
		return ErrInvalidInput
	}
	m.Content += delta
	m.UpdatedAt = time.Now()
	return nil
}