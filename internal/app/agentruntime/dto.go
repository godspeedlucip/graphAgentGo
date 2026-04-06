package agentruntime

import (
	domain "go-sse-skeleton/internal/domain/agentruntime"
	port "go-sse-skeleton/internal/port/agentruntime"
)

type BuildRuntimeCommand struct {
	AgentID   string
	SessionID string
}

type BuildRuntimeResult struct {
	Runtime port.Runtime
	Spec    domain.RuntimeSpec
}
