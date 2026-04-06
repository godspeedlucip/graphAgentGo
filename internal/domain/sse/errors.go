package sse

import "errors"

var (
	ErrClientNotFound    = errors.New("sse client not found")
	ErrClientDisconnected = errors.New("sse client disconnected")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidMessage    = errors.New("invalid sse message")
)