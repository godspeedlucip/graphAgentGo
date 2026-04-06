package orchestration

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/api/orchestration/execute", h.Execute)
}
