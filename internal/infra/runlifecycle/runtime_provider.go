package runlifecycle

import (
	"context"
	"errors"

	factoryapp "go-sse-skeleton/internal/app/agentruntime"
	port "go-sse-skeleton/internal/port/runlifecycle"
)

type FactoryRuntimeProvider struct {
	factory factoryapp.FactoryService
}

func NewFactoryRuntimeProvider(factory factoryapp.FactoryService) (*FactoryRuntimeProvider, error) {
	if factory == nil {
		return nil, errors.New("nil runtime factory")
	}
	return &FactoryRuntimeProvider{factory: factory}, nil
}

func (p *FactoryRuntimeProvider) GetRuntime(ctx context.Context, agentID string, sessionID string) (port.Runtime, error) {
	result, err := p.factory.Build(ctx, factoryapp.BuildRuntimeCommand{AgentID: agentID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return runtimeAdapter{base: result.Runtime}, nil
}

type runtimeAdapter struct {
	base portRuntime
}

type portRuntime interface {
	Run(ctx context.Context) error
	AgentID() string
	SessionID() string
}

func (r runtimeAdapter) Run(ctx context.Context, in port.RuntimeInput) (port.RuntimeOutput, error) {
	_ = in
	// TODO: map RuntimeInput.UserInput/Metadata into graph execution context once runtime supports explicit input payload.
	if err := r.base.Run(ctx); err != nil {
		return port.RuntimeOutput{}, err
	}
	return port.RuntimeOutput{}, nil
}
