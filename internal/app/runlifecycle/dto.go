package runlifecycle

import (
	domain "go-sse-skeleton/internal/domain/runlifecycle"
	port "go-sse-skeleton/internal/port/runlifecycle"
)

type StartRunCommand struct {
	RunID     string
	AgentID   string
	SessionID string
	UserInput string
	Metadata  map[string]any
}

type CancelRunCommand struct {
	RunID string
	Cause string
}

type RunResult struct {
	RunID     string
	Status    domain.Status
	Output    string
	ErrorCode string
	ErrorMsg  string
}

type RuntimeAdapter struct {
	Runtime port.Runtime
}
