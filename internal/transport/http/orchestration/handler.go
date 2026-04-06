package orchestration

import (
	"encoding/json"
	"errors"
	"net/http"

	app "go-sse-skeleton/internal/app/orchestration"
	domain "go-sse-skeleton/internal/domain/orchestration"
)

type Handler struct {
	service app.Service
}

func NewHandler(service app.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("nil orchestration service")
	}
	return &Handler{service: service}, nil
}

func (h *Handler) Execute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID        string         `json:"runId"`
		AgentID      string         `json:"agentId"`
		SessionID    string         `json:"sessionId"`
		UserInput    string         `json:"userInput"`
		SystemPrompt string         `json:"systemPrompt"`
		MaxSteps     int            `json:"maxSteps"`
		Metadata     map[string]any `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.Execute(r.Context(), app.ExecuteGraphCommand{Input: domain.GraphInput{
		RunID:        req.RunID,
		AgentID:      req.AgentID,
		SessionID:    req.SessionID,
		UserInput:    req.UserInput,
		SystemPrompt: req.SystemPrompt,
		MaxSteps:     req.MaxSteps,
		Metadata:     req.Metadata,
	}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
