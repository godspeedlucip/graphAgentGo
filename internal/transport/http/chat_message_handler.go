package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	app "go-sse-skeleton/internal/app/chat"
	domain "go-sse-skeleton/internal/domain/chat"
)

type ChatMessageHandler struct {
	service app.Service
}

func NewChatMessageHandler(service app.Service) (*ChatMessageHandler, error) {
	if service == nil {
		return nil, errors.New("nil chat message service")
	}
	return &ChatMessageHandler{service: service}, nil
}

type createChatMessageRequest struct {
	AgentID   string           `json:"agentId"`
	SessionID string           `json:"sessionId"`
	Role      domain.Role      `json:"role"`
	Content   string           `json:"content"`
	Metadata  *domain.Metadata `json:"metadata"`
}

type updateChatMessageRequest struct {
	Content  *string          `json:"content"`
	Metadata *domain.Metadata `json:"metadata"`
}

func (h *ChatMessageHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	id, err := h.service.CreateFromCommand(r.Context(), app.CreateMessageCommand{
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		Role:      req.Role,
		Content:   req.Content,
		Metadata:  req.Metadata,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeSuccess(w, http.StatusOK, map[string]any{"chatMessageId": id})
}

func (h *ChatMessageHandler) ListBySession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/chat-messages/session/")
	if sessionID == "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	messages, err := h.service.ListBySession(r.Context(), sessionID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"chatMessages": messages})
}

func (h *ChatMessageHandler) ListRecentBySession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	limitText := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitText)
	if limit <= 0 {
		limit = 20
	}

	messages, err := h.service.ListRecentBySession(r.Context(), sessionID, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"chatMessages": messages})
}

func (h *ChatMessageHandler) Append(w http.ResponseWriter, r *http.Request) {
	messageID := parseChatMessageID(r.URL.Path)
	if messageID == "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	var body struct {
		AppendContent string `json:"appendContent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}

	if err := h.service.Append(r.Context(), messageID, body.AppendContent); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"chatMessageId": messageID})
}

func (h *ChatMessageHandler) Update(w http.ResponseWriter, r *http.Request) {
	messageID := parseChatMessageID(r.URL.Path)
	if messageID == "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	var req updateChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	if err := h.service.Update(r.Context(), messageID, app.UpdateMessageCommand{
		Content:  req.Content,
		Metadata: req.Metadata,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess[any](w, http.StatusOK, nil)
}

func (h *ChatMessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	messageID := parseChatMessageID(r.URL.Path)
	if messageID == "" {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	if err := h.service.Delete(r.Context(), messageID); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess[any](w, http.StatusOK, nil)
}

func parseChatMessageID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/chat-messages/")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/append") {
		trimmed = strings.TrimSuffix(trimmed, "/append")
		trimmed = strings.Trim(trimmed, "/")
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return strings.Trim(trimmed[:idx], "/")
	}
	return trimmed
}
