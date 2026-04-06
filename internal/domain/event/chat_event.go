package event

type ChatEvent struct {
	EventID   string `json:"eventId,omitempty"`
	AgentID   string `json:"agentId"`
	SessionID string `json:"sessionId"`
	UserInput string `json:"userInput"`
}

func (e ChatEvent) Validate() error {
	if e.AgentID == "" || e.SessionID == "" {
		return ErrInvalidEvent
	}
	return nil
}