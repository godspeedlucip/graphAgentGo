package memory

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /internal/memory/", h.Get)
	mux.HandleFunc("DELETE /internal/memory/", h.Clear)
}