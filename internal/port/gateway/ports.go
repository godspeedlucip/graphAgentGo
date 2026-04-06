package gateway

import (
	"context"
	"net/http"

	domain "go-sse-skeleton/internal/domain/gateway"
)

type DecisionEngine interface {
	Decide(ctx context.Context, req domain.Request, rules domain.Rules) (domain.Decision, error)
}

type RulesProvider interface {
	Current(ctx context.Context) (domain.Rules, error)
}

type UpstreamProxy interface {
	ForwardHTTP(ctx context.Context, target domain.Target, req *http.Request) (*http.Response, error)
	ForwardSSE(ctx context.Context, target domain.Target, w http.ResponseWriter, req *http.Request) error
}

type ResponseCompat interface {
	WriteError(w http.ResponseWriter, err error)
	CopyUpstreamResponse(w http.ResponseWriter, resp *http.Response) error
}