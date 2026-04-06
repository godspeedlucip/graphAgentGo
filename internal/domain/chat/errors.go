package chat

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("chat message not found")
)

type ErrorCode string

const (
	ErrorCodeInvalidInput ErrorCode = "CHAT_MESSAGE_INVALID_INPUT"
	ErrorCodeNotFound     ErrorCode = "CHAT_MESSAGE_NOT_FOUND"
	ErrorCodeInternal     ErrorCode = "CHAT_MESSAGE_INTERNAL_ERROR"
)

func CodeOf(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrInvalidInput):
		return ErrorCodeInvalidInput
	case errors.Is(err, ErrNotFound):
		return ErrorCodeNotFound
	default:
		return ErrorCodeInternal
	}
}
