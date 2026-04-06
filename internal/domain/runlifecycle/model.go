package runlifecycle

import "time"

type Status string

const (
	StatusInit     Status = "init"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
	StatusTimeout  Status = "timeout"
)

type RunRecord struct {
	RunID      string
	AgentID    string
	SessionID  string
	Status     Status
	Input      string
	Output     string
	ErrorCode  string
	ErrorMsg   string
	Metadata   map[string]any
	StartedAt  time.Time
	FinishedAt *time.Time
}

type LifecycleEvent struct {
	RunID      string
	AgentID    string
	SessionID  string
	Status     Status
	OccurredAt time.Time
	Metadata   map[string]any
}
