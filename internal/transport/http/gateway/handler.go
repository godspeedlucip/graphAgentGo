package gateway

import (
	"errors"
	"net/http"
	"strings"

	app "go-sse-skeleton/internal/app/gateway"
)

type Handler struct {
	service app.Service
}

func NewHandler(service app.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("nil gateway service")
	}
	return &Handler{service: service}, nil
}

func (h *Handler) Unified(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/sse/") {
		_, err := h.service.ForwardSSE(r.Context(), w, r)
		if err != nil {
			h.service.WriteError(w, err)
		}
		return
	}

	resp, _, err := h.service.ForwardHTTP(r.Context(), r)
	if err != nil {
		h.service.WriteError(w, err)
		return
	}
	defer resp.Body.Close()
	_ = h.service.CopyUpstreamResponse(w, resp)
}
