package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

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
) (Service, error) {
	if decisionEngine == nil || rulesProvider == nil || proxy == nil || compat == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in gateway service")
	}
	return &service{
		decisionEngine: decisionEngine,
		rulesProvider:  rulesProvider,
		proxy:          proxy,
		compat:         compat,
		messageStore:   messageStore,
		eventBus:       eventBus,
		sseNotifier:    sseNotifier,
		llmClient:      llmClient,
	}, nil
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
	resp, err := s.proxy.ForwardHTTP(ctx, decision.Target, req)
	if err != nil {
		// Keep core compatibility: allow safe-method fallback from Go -> Java on proxy failure.
		// TODO: evaluate fallback policy per endpoint; non-idempotent routes should be opt-in.
		if decision.Target == domain.TargetGo && isSafeMethod(req.Method) {
			slog.Warn("gateway fallback to java", "method", req.Method, "path", req.URL.Path, "err", err)
			fallbackResp, fallbackErr := s.proxy.ForwardHTTP(ctx, domain.TargetJava, req)
			if fallbackErr == nil {
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
	if err = s.proxy.ForwardSSE(ctx, decision.Target, w, req); err != nil {
		// SSE fallback can only happen before first byte is sent.
		// Current implementation retries only when initial proxy call returns error.
		if decision.Target == domain.TargetGo {
			slog.Warn("gateway sse fallback to java", "path", req.URL.Path, "err", err)
			fallbackErr := s.proxy.ForwardSSE(ctx, domain.TargetJava, w, req)
			if fallbackErr == nil {
				return domain.Decision{
					Target: domain.TargetJava,
					Reason: decision.Reason + "_fallback_java",
				}, nil
			}
		}
		return decision, err
	}
	return decision, nil
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
