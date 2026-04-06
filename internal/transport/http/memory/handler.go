package memory

import (
	"errors"

	app "go-sse-skeleton/internal/app/memory"
)

type Handler struct {
	service app.Service
}

func NewHandler(service app.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("nil memory service")
	}
	return &Handler{service: service}, nil
}
