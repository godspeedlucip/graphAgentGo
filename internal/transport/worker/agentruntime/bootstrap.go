package agentruntime

import (
	"context"
	"errors"

	app "go-sse-skeleton/internal/app/agentruntime"
)

type Bootstrap struct {
	factory app.FactoryService
}

func NewBootstrap(factory app.FactoryService) (*Bootstrap, error) {
	if factory == nil {
		return nil, errors.New("nil runtime factory")
	}
	return &Bootstrap{factory: factory}, nil
}

func (b *Bootstrap) BuildAndRun(ctx context.Context, agentID string, sessionID string) error {
	result, err := b.factory.Build(ctx, app.BuildRuntimeCommand{AgentID: agentID, SessionID: sessionID})
	if err != nil {
		return err
	}
	return result.Runtime.Run(ctx)
}