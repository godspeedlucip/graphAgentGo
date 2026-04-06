package vector

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /internal/vector/search", h.SimilaritySearchByKB)
	mux.HandleFunc("GET /internal/vector/search-all", h.SimilaritySearchAllKB)
	mux.HandleFunc("GET /internal/vector/health", h.Health)
}