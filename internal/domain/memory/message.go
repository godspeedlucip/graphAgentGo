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
	ToolResponse *ToolResponse `json:"toolResponse,omitempty"`
	ToolCalls    []ToolCall    `json:"toolCalls,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolResponse struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	ResponseData string `json:"responseData,omitempty"`
}

type CachedMessage struct {
	Role     Role      `json:"role"`
	Content  string    `json:"content"`
	Metadata *Metadata `json:"metadata,omitempty"`
}
