package http

import "net/http"

func RegisterChatMessageRoutes(mux *http.ServeMux, h *ChatMessageHandler) {
	mux.HandleFunc("GET /api/chat-messages/session/", h.ListBySession)
	mux.HandleFunc("GET /api/chat-messages/recent", h.ListRecentBySession)
	mux.HandleFunc("POST /api/chat-messages", h.Create)
	// Internal append endpoint for streaming assembly; supports /{id} and /{id}/append in handler parsing.
	mux.HandleFunc("POST /api/chat-messages/", h.Append)
	mux.HandleFunc("PATCH /api/chat-messages/", h.Update)
	mux.HandleFunc("DELETE /api/chat-messages/", h.Delete)
}
