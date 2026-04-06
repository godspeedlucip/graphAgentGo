package chat

import (
	"errors"
	"fmt"

	domain "go-sse-skeleton/internal/domain/chat"
)

type ErrorCode string

const (
	ErrorCodeInvalidInput ErrorCode = ErrorCode(domain.ErrorCodeInvalidInput)
	ErrorCodeNotFound     ErrorCode = ErrorCode(domain.ErrorCodeNotFound)
	ErrorCodeInternal     ErrorCode = ErrorCode(domain.ErrorCodeInternal)
)

type AppError struct {
	Code ErrorCode
	Err  error
}

func (e *AppError) Error() string {
	if e == nil || e.Err == nil {
		return "chat app error"
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapAppError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return err
	}
	switch domain.CodeOf(err) {
	case domain.ErrorCodeInvalidInput:
		return &AppError{Code: ErrorCodeInvalidInput, Err: err}
	case domain.ErrorCodeNotFound:
		return &AppError{Code: ErrorCodeNotFound, Err: err}
	default:
		return &AppError{Code: ErrorCodeInternal, Err: err}
	}
}

func newInvalidInputError(message string) error {
	return &AppError{
		Code: ErrorCodeInvalidInput,
		Err:  fmt.Errorf("%w: %s", domain.ErrInvalidInput, message),
	}
}
