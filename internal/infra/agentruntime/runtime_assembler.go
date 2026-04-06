package agentruntime

import (
	"context"
	"errors"

	app "go-sse-skeleton/internal/app/agentruntime"
	port "go-sse-skeleton/internal/port/agentruntime"
)

type SimpleRuntime struct {
	agentID   string
	sessionID string
	graph     port.GraphRuntime
}

func NewSimpleRuntime(agentID string, sessionID string, graph port.GraphRuntime) (*SimpleRuntime, error) {
	if agentID == "" || sessionID == "" || graph == nil {
		return nil, errors.New("invalid runtime constructor input")
	}
	return &SimpleRuntime{agentID: agentID, sessionID: sessionID, graph: graph}, nil
}

func (r *SimpleRuntime) Run(ctx context.Context) error {
	return r.graph.Execute(ctx)
}

func (r *SimpleRuntime) AgentID() string {
	return r.agentID
}

func (r *SimpleRuntime) SessionID() string {
	return r.sessionID
}

type RuntimeAssembler struct{}

func NewRuntimeAssembler() *RuntimeAssembler {
	return &RuntimeAssembler{}
}

func (a *RuntimeAssembler) Assemble(_ context.Context, in app.RuntimeAssembleInput) (port.Runtime, error) {
	return NewSimpleRuntime(in.AgentID, in.SessionID, in.Graph)
}
