package runlifecycle

import (
	"encoding/json"
	"errors"
	"net/http"

	app "go-sse-skeleton/internal/app/runlifecycle"
)

type Handler struct {
	service app.Service
}

func NewHandler(service app.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("nil run lifecycle service")
	}
	return &Handler{service: service}, nil
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID     string         `json:"runId"`
		AgentID   string         `json:"agentId"`
		SessionID string         `json:"sessionId"`
		UserInput string         `json:"userInput"`
		Metadata  map[string]any `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.Start(r.Context(), app.StartRunCommand{
		RunID:     req.RunID,
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		UserInput: req.UserInput,
		Metadata:  req.Metadata,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("runId")
	result, err := h.service.Get(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID string `json:"runId"`
		Cause string `json:"cause"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.service.Cancel(r.Context(), app.CancelRunCommand{RunID: req.RunID, Cause: req.Cause}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
