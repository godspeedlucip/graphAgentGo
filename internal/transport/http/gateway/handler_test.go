package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "go-sse-skeleton/internal/app/gateway"
	domain "go-sse-skeleton/internal/domain/gateway"
)

type fakeGatewayService struct {
	forwardHTTPResp *http.Response
	forwardHTTPErr  error
	forwardSSEErr   error

	writeErrorCalls int
	copyCalls       int
}

func (f *fakeGatewayService) Route(context.Context, app.RouteRequest) (app.RouteResult, error) {
	return app.RouteResult{}, nil
}

func (f *fakeGatewayService) ForwardHTTP(context.Context, *http.Request) (*http.Response, domain.Decision, error) {
	return f.forwardHTTPResp, domain.Decision{Target: domain.TargetGo, Reason: "test"}, f.forwardHTTPErr
}

func (f *fakeGatewayService) ForwardSSE(context.Context, http.ResponseWriter, *http.Request) (domain.Decision, error) {
	return domain.Decision{Target: domain.TargetGo, Reason: "test"}, f.forwardSSEErr
}

func (f *fakeGatewayService) WriteError(w http.ResponseWriter, _ error) {
	f.writeErrorCalls++
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(`{"message":"compat"}`))
}

func (f *fakeGatewayService) CopyUpstreamResponse(w http.ResponseWriter, resp *http.Response) error {
	f.copyCalls++
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err := io.Copy(w, resp.Body)
	return err
}

func TestHandlerUnifiedHTTPUsesCompatCopy(t *testing.T) {
	t.Parallel()

	svc := &fakeGatewayService{
		forwardHTTPResp: &http.Response{
			StatusCode: http.StatusCreated,
			Header:     http.Header{"X-Upstream": []string{"go"}},
			Body:       io.NopCloser(strings.NewReader("ok")),
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/messages", nil)
	rec := httptest.NewRecorder()
	h.Unified(rec, req)

	if svc.copyCalls != 1 {
		t.Fatalf("expected compat copy to be called once, got %d", svc.copyCalls)
	}
	if rec.Code != http.StatusCreated || rec.Body.String() != "ok" {
		t.Fatalf("unexpected upstream response passthrough: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandlerUnifiedErrorUsesCompatWriter(t *testing.T) {
	t.Parallel()

	svc := &fakeGatewayService{forwardHTTPErr: errors.New("proxy failed")}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/messages", nil)
	rec := httptest.NewRecorder()
	h.Unified(rec, req)

	if svc.writeErrorCalls != 1 {
		t.Fatalf("expected compat error writer call once, got %d", svc.writeErrorCalls)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestHandlerUnifiedSSEErrorUsesCompatWriter(t *testing.T) {
	t.Parallel()

	svc := &fakeGatewayService{forwardSSEErr: errors.New("stream failed")}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sse/connect/s1", nil)
	rec := httptest.NewRecorder()
	h.Unified(rec, req)

	if svc.writeErrorCalls != 1 {
		t.Fatalf("expected compat error writer call once, got %d", svc.writeErrorCalls)
	}
}
