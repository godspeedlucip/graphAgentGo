package vector

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	app "go-sse-skeleton/internal/app/vector"
)

type Handler struct {
	service app.Service
}

func NewHandler(service app.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("nil vector service")
	}
	return &Handler{service: service}, nil
}

func (h *Handler) SimilaritySearchByKB(w http.ResponseWriter, r *http.Request) {
	kbID := r.URL.Query().Get("kbId")
	query := r.URL.Query().Get("query")
	limitText := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitText)

	result, err := h.service.SimilaritySearch(r.Context(), kbID, query, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"results": result})
}

func (h *Handler) SimilaritySearchAllKB(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	limitText := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitText)

	result, err := h.service.SimilaritySearchAllKnowledgeBases(r.Context(), query, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"results": result})
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(strings.TrimSpace("ok")))
}