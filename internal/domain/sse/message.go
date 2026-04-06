package sse

type Type string

const (
	TypeAIGeneratedContent      Type = "AI_GENERATED_CONTENT"
	TypeAIGeneratedContentStart Type = "AI_GENERATED_CONTENT_START"
	TypeAIGeneratedContentDelta Type = "AI_GENERATED_CONTENT_DELTA"
	TypeAIGeneratedContentEnd   Type = "AI_GENERATED_CONTENT_END"
	TypeAIPlanning              Type = "AI_PLANNING"
	TypeAIThinking              Type = "AI_THINKING"
	TypeAIExecuting             Type = "AI_EXECUTING"
	TypeAIDone                  Type = "AI_DONE"
)

type Message struct {
	Type     Type     `json:"type"`
	Payload  Payload  `json:"payload"`
	Metadata Metadata `json:"metadata"`
}

type Payload struct {
	Message      any    `json:"message,omitempty"`
	DeltaContent string `json:"deltaContent,omitempty"`
	StatusText   string `json:"statusText,omitempty"`
	Done         *bool  `json:"done,omitempty"`
}

type Metadata struct {
	ChatMessageID string `json:"chatMessageId,omitempty"`
}

func (m Message) Validate() error {
	if m.Type == "" {
		return ErrInvalidMessage
	}
	return nil
}