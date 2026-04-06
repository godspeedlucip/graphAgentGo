package agentruntime

type AgentConfig struct {
	AgentID      string
	Model        string
	SystemPrompt string
	AllowedTools []string
	AllowedKBs   []string
	MaxMessages  int
}

type RuntimeSpec struct {
	AgentID      string
	SessionID    string
	Model        string
	SystemPrompt string
}

type ToolDef struct {
	Name string
	Kind string // rag | sql | tool
}

type KnowledgeBase struct {
	ID   string
	Name string
}