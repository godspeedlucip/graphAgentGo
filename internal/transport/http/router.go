package http

import "net/http"

func RegisterRoutes(mux *http.ServeMux, sseHandler *SSEHandler) {
	mux.HandleFunc("GET /sse/connect/", sseHandler.Connect)
}