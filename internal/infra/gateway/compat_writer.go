package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type CompatWriter struct{}

func NewCompatWriter() *CompatWriter {
	return &CompatWriter{}
}

func (w *CompatWriter) WriteError(rw http.ResponseWriter, err error) {
	status, code, msg := mapGatewayError(err)
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"code":      status,
		"message":   msg,
		"data":      nil,
		"errorCode": code,
	})
}

func (w *CompatWriter) CopyUpstreamResponse(rw http.ResponseWriter, resp *http.Response) error {
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	_, err := io.Copy(rw, resp.Body)
	return err
}

func mapGatewayError(err error) (status int, code string, msg string) {
	switch {
	case err == nil:
		return http.StatusOK, "", "success"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "GATEWAY_INVALID_INPUT", err.Error()
	case errors.Is(err, domain.ErrNoRouteMatched):
		return http.StatusNotFound, "GATEWAY_NO_ROUTE", err.Error()
	case errors.Is(err, domain.ErrBodyTooLarge):
		return http.StatusRequestEntityTooLarge, "GATEWAY_BODY_TOO_LARGE", err.Error()
	default:
		return http.StatusBadGateway, "GATEWAY_UPSTREAM_ERROR", "upstream unavailable"
	}
}
