package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	domain "go-sse-skeleton/internal/domain/gateway"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
	port "go-sse-skeleton/internal/port/gateway"
)

type Service interface {
	Route(ctx context.Context, req RouteRequest) (RouteResult, error)
	ForwardHTTP(ctx context.Context, req *http.Request) (*http.Response, domain.Decision, error)
	ForwardSSE(ctx context.Context, w http.ResponseWriter, req *http.Request) (domain.Decision, error)
	WriteError(w http.ResponseWriter, err error)
	CopyUpstreamResponse(w http.ResponseWriter, resp *http.Response) error
}

type service struct {
	decisionEngine port.DecisionEngine
	rulesProvider  port.RulesProvider
	proxy          port.UpstreamProxy
	compat         port.ResponseCompat

	// Shared dependencies are injected to keep architecture contracts consistent.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client
	observer     Observer
}

func NewService(
	decisionEngine port.DecisionEngine,
	rulesProvider port.RulesProvider,
	proxy port.UpstreamProxy,
	compat port.ResponseCompat,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
	opts ...Option,
) (Service, error) {
	if decisionEngine == nil || rulesProvider == nil || proxy == nil || compat == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in gateway service")
	}
	svc := &service{
		decisionEngine: decisionEngine,
		rulesProvider:  rulesProvider,
		proxy:          proxy,
		compat:         compat,
		messageStore:   messageStore,
		eventBus:       eventBus,
		sseNotifier:    sseNotifier,
		llmClient:      llmClient,
		observer:       NewNoopObserver(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if svc.observer == nil {
		svc.observer = NewNoopObserver()
	}
	return svc, nil
}

func (s *service) Route(ctx context.Context, req RouteRequest) (RouteResult, error) {
	if req.Method == "" || req.Path == "" {
		return RouteResult{}, domain.ErrInvalidInput
	}
	rules, err := s.rulesProvider.Current(ctx)
	if err != nil {
		return RouteResult{}, err
	}
	decision, err := s.decisionEngine.Decide(ctx, domain.Request{
		Method: req.Method,
		Path:   req.Path,
		Header: req.Header,
	}, rules)
	if err != nil {
		return RouteResult{}, err
	}
	return RouteResult{Target: string(decision.Target), Reason: decision.Reason}, nil
}

func (s *service) ForwardHTTP(ctx context.Context, req *http.Request) (*http.Response, domain.Decision, error) {
	if req == nil {
		return nil, domain.Decision{}, domain.ErrInvalidInput
	}
	rules, err := s.rulesProvider.Current(ctx)
	if err != nil {
		return nil, domain.Decision{}, err
	}
	decision, err := s.decisionEngine.Decide(ctx, domain.Request{
		Method: req.Method,
		Path:   req.URL.Path,
		Header: copyHeaderMap(req.Header),
	}, rules)
	if err != nil {
		return nil, domain.Decision{}, err
	}
	s.observer.RecordTargetHit(decision.Target, decision.Reason)
	replayBody, replayable, err := snapshotBodyForReplay(req, 1<<20)
	if err != nil {
		return nil, decision, err
	}
	reqPrimary := cloneRequestWithBody(ctx, req, replayBody, replayable)
	resp, err := s.proxy.ForwardHTTP(ctx, decision.Target, reqPrimary)
	if err != nil {
		if decision.Target == domain.TargetGo && s.canFallbackToJava(req, rules, replayable) {
			slog.Warn("gateway fallback to java", "method", req.Method, "path", req.URL.Path, "err", err)
			reqFallback := cloneRequestWithBody(ctx, req, replayBody, replayable)
			fallbackResp, fallbackErr := s.proxy.ForwardHTTP(ctx, domain.TargetJava, reqFallback)
			if fallbackErr == nil {
				s.observer.RecordFallback(decision.Target, domain.TargetJava, "proxy_error", req.Method, req.URL.Path)
				return fallbackResp, domain.Decision{
					Target: domain.TargetJava,
					Reason: decision.Reason + "_fallback_java",
				}, nil
			}
		}
		return nil, decision, err
	}
	return resp, decision, nil
}

func (s *service) ForwardSSE(ctx context.Context, w http.ResponseWriter, req *http.Request) (domain.Decision, error) {
	if req == nil || w == nil {
		return domain.Decision{}, domain.ErrInvalidInput
	}
	rules, err := s.rulesProvider.Current(ctx)
	if err != nil {
		return domain.Decision{}, err
	}
	decision, err := s.decisionEngine.Decide(ctx, domain.Request{
		Method: req.Method,
		Path:   req.URL.Path,
		Header: copyHeaderMap(req.Header),
	}, rules)
	if err != nil {
		return domain.Decision{}, err
	}
	s.observer.RecordTargetHit(decision.Target, decision.Reason)
	if err = s.proxy.ForwardSSE(ctx, decision.Target, w, req); err != nil {
		// SSE fallback can only happen before first byte is sent.
		// Current implementation retries only when initial proxy call returns error.
		if decision.Target == domain.TargetGo {
			slog.Warn("gateway sse fallback to java", "path", req.URL.Path, "err", err)
			s.observer.RecordSSEDisconnect(decision.Target, "proxy_error")
			fallbackErr := s.proxy.ForwardSSE(ctx, domain.TargetJava, w, req)
			if fallbackErr == nil {
				s.observer.RecordFallback(decision.Target, domain.TargetJava, "sse_proxy_error", req.Method, req.URL.Path)
				return domain.Decision{
					Target: domain.TargetJava,
					Reason: decision.Reason + "_fallback_java",
				}, nil
			}
		}
		s.observer.RecordSSEDisconnect(decision.Target, "proxy_error")
		return decision, err
	}
	return decision, nil
}

func (s *service) WriteError(w http.ResponseWriter, err error) {
	s.compat.WriteError(w, err)
}

func (s *service) CopyUpstreamResponse(w http.ResponseWriter, resp *http.Response) error {
	return s.compat.CopyUpstreamResponse(w, resp)
}

func copyHeaderMap(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) == 0 {
			continue
		}
		out[k] = v[0]
	}
	return out
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func (s *service) canFallbackToJava(req *http.Request, rules domain.Rules, replayable bool) bool {
	if req == nil {
		return false
	}
	if isSafeMethod(req.Method) {
		return true
	}
	if !replayable {
		return false
	}
	if !methodInWhitelist(req.Method, rules.WriteFallbackMethods) {
		return false
	}
	if !pathInWhitelist(req.URL.Path, rules.WriteFallbackPathPrefixes) {
		return false
	}
	keyHeader := rules.IdempotencyHeader
	if strings.TrimSpace(keyHeader) == "" {
		keyHeader = "Idempotency-Key"
	}
	idempotencyKey := strings.TrimSpace(req.Header.Get(keyHeader))
	return idempotencyKey != ""
}

func methodInWhitelist(method string, whitelist []string) bool {
	for _, m := range whitelist {
		if strings.EqualFold(strings.TrimSpace(m), method) {
			return true
		}
	}
	return false
}

func pathInWhitelist(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, strings.TrimSpace(prefix)) {
			return true
		}
	}
	return false
}

func snapshotBodyForReplay(req *http.Request, max int64) (body []byte, replayable bool, err error) {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil, true, nil
	}
	// Only cache body when size is bounded and safe to keep in memory.
	if req.ContentLength < 0 {
		return nil, false, nil
	}
	if req.ContentLength > max {
		return nil, false, nil
	}
	b, readErr := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if readErr != nil {
		return nil, false, readErr
	}
	if int64(len(b)) > max {
		return nil, false, nil
	}
	req.Body = io.NopCloser(bytes.NewReader(b))
	return b, true, nil
}

func cloneRequestWithBody(ctx context.Context, req *http.Request, body []byte, replayable bool) *http.Request {
	cloned := req.Clone(ctx)
	if !replayable {
		cloned.Body = req.Body
		return cloned
	}
	if body == nil {
		cloned.Body = http.NoBody
		cloned.ContentLength = 0
		return cloned
	}
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.ContentLength = int64(len(body))
	return cloned
}
