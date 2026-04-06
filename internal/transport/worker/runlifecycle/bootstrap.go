package runlifecycle

import (
	"context"
	"errors"

	app "go-sse-skeleton/internal/app/runlifecycle"
)

type Bootstrap struct {
	service app.Service
}

func NewBootstrap(service app.Service) (*Bootstrap, error) {
	if service == nil {
		return nil, errors.New("nil run lifecycle service")
	}
	return &Bootstrap{service: service}, nil
}

func (b *Bootstrap) StartRun(ctx context.Context, cmd app.StartRunCommand) (app.RunResult, error) {
	return b.service.Start(ctx, cmd)
}

func (b *Bootstrap) CancelRun(ctx context.Context, cmd app.CancelRunCommand) error {
	return b.service.Cancel(ctx, cmd)
}
