package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "go-sse-skeleton/internal/domain/gateway"
)

func TestCompatWriterWriteErrorContract(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	NewCompatWriter().WriteError(rec, domain.ErrInvalidInput)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %s", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["code"] != float64(http.StatusBadRequest) {
		t.Fatalf("unexpected code field: %+v", body)
	}
	if body["errorCode"] != "GATEWAY_INVALID_INPUT" {
		t.Fatalf("unexpected errorCode field: %+v", body)
	}
	if _, ok := body["data"]; !ok {
		t.Fatalf("expected data field in error contract: %+v", body)
	}
}
