package runlifecycle

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/api/runs/start", h.Start)
	mux.HandleFunc("/api/runs/get", h.Get)
	mux.HandleFunc("/api/runs/cancel", h.Cancel)
}
