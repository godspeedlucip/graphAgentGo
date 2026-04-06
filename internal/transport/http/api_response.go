package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	appchat "go-sse-skeleton/internal/app/chat"
	domainchat "go-sse-skeleton/internal/domain/chat"
)

type apiResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	ErrorCode string `json:"errorCode,omitempty"`
}

func writeSuccess[T any](w http.ResponseWriter, status int, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse[T]{
		Code:    status,
		Message: "success",
		Data:    data,
	})
}

func writeError(w http.ResponseWriter, err error) {
	status, errCode, msg := mapHTTPError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse[any]{
		Code:      status,
		Message:   msg,
		Data:      nil,
		ErrorCode: errCode,
	})
}

func mapHTTPError(err error) (status int, errCode string, msg string) {
	if err == nil {
		return http.StatusOK, "", "success"
	}
	var appErr *appchat.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case appchat.ErrorCodeInvalidInput:
			return http.StatusBadRequest, string(appchat.ErrorCodeInvalidInput), appErr.Error()
		case appchat.ErrorCodeNotFound:
			return http.StatusNotFound, string(appchat.ErrorCodeNotFound), appErr.Error()
		default:
			return http.StatusInternalServerError, string(appchat.ErrorCodeInternal), "internal server error"
		}
	}

	switch domainchat.CodeOf(err) {
	case domainchat.ErrorCodeInvalidInput:
		return http.StatusBadRequest, string(domainchat.ErrorCodeInvalidInput), err.Error()
	case domainchat.ErrorCodeNotFound:
		return http.StatusNotFound, string(domainchat.ErrorCodeNotFound), err.Error()
	default:
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr), errors.As(err, &typeErr), errors.Is(err, io.EOF):
			return http.StatusBadRequest, string(domainchat.ErrorCodeInvalidInput), "invalid request body"
		}
		return http.StatusInternalServerError, string(domainchat.ErrorCodeInternal), "internal server error"
	}
}
