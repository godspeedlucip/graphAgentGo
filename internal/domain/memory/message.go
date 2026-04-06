package memory

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role     Role
	Content  string
	Metadata *Metadata
}

type Metadata struct {
	ToolResponse any   `json:"toolResponse,omitempty"`
	ToolCalls    []any `json:"toolCalls,omitempty"`
}

type CachedMessage struct {
	Role     Role      `json:"role"`
	Content  string    `json:"content"`
	Metadata *Metadata `json:"metadata,omitempty"`
}