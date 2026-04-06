package orchestration

import (
	"context"
	"errors"

	app "go-sse-skeleton/internal/app/orchestration"
)

type Bootstrap struct {
	service app.Service
}

func NewBootstrap(service app.Service) (*Bootstrap, error) {
	if service == nil {
		return nil, errors.New("nil orchestration service")
	}
	return &Bootstrap{service: service}, nil
}

func (b *Bootstrap) Execute(ctx context.Context, cmd app.ExecuteGraphCommand) (app.ExecuteGraphResult, error) {
	return b.service.Execute(ctx, cmd)
}
