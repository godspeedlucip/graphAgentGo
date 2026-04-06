package runlifecycle

import (
	"bytes"
	"context"
	"errors"

	factoryapp "go-sse-skeleton/internal/app/agentruntime"
	agport "go-sse-skeleton/internal/port/agentruntime"
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
	runCtx := ctx
	var out bytes.Buffer
	if in.AppendOutput != nil {
		runCtx = agport.WithExecutionHooks(ctx, agport.ExecutionHooks{
			AppendDelta: func(deltaCtx context.Context, delta string) error {
				return in.AppendOutput(deltaCtx, delta)
			},
			CollectDelta: func(delta string) {
				_, _ = out.WriteString(delta)
			},
		})
	}
	// TODO: map RuntimeInput.UserInput/Metadata into richer runtime execution context fields.
	if err := r.base.Run(runCtx); err != nil {
		return port.RuntimeOutput{}, err
	}
	return port.RuntimeOutput{Text: out.String()}, nil
}
