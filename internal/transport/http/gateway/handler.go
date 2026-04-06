package gateway

import (
	"errors"
	"io"
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
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	resp, _, err := h.service.ForwardHTTP(r.Context(), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
