package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainchat "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/gateway"
)

type fakeDecisionEngine struct {
	decision domain.Decision
	err      error
}

func (f fakeDecisionEngine) Decide(context.Context, domain.Request, domain.Rules) (domain.Decision, error) {
	if f.err != nil {
		return domain.Decision{}, f.err
	}
	return f.decision, nil
}

type fakeRulesProvider struct {
	rules domain.Rules
	err   error
}

func (f fakeRulesProvider) Current(context.Context) (domain.Rules, error) {
	if f.err != nil {
		return domain.Rules{}, f.err
	}
	return f.rules, nil
}

type fakeCompat struct{}

func (fakeCompat) WriteError(http.ResponseWriter, error) {}

func (fakeCompat) CopyUpstreamResponse(http.ResponseWriter, *http.Response) error { return nil }

type fakeStore struct{}

func (fakeStore) Create(context.Context, *domainchat.Message) (string, error) { return "", nil }

func (fakeStore) GetByID(context.Context, string) (*domainchat.Message, error) { return nil, nil }

func (fakeStore) ListBySession(context.Context, string) ([]*domainchat.Message, error) { return nil, nil }

func (fakeStore) ListRecentBySession(context.Context, string, int) ([]*domainchat.Message, error) {
	return nil, nil
}

func (fakeStore) Update(context.Context, *domainchat.Message) error { return nil }

func (fakeStore) Delete(context.Context, string) error { return nil }

type fakeQueue struct{}

func (fakeQueue) Publish(context.Context, string, any) error { return nil }

type fakeNotifier struct{}

func (fakeNotifier) NotifyDone(context.Context, string) error { return nil }

type fakeLLM struct{}

func (fakeLLM) Generate(context.Context, string) (string, error) { return "", nil }

type fakeObserver struct {
	targetHits     int
	fallbacks      int
	sseDisconnects int
}

func (o *fakeObserver) RecordTargetHit(domain.Target, string) { o.targetHits++ }

func (o *fakeObserver) RecordFallback(domain.Target, domain.Target, string, string, string) {
	o.fallbacks++
}

func (o *fakeObserver) RecordSSEDisconnect(domain.Target, string) { o.sseDisconnects++ }

type fakeProxy struct {
	httpBodies     []string
	httpTargets    []domain.Target
	httpCall       int
	httpErrByCall  map[int]error
	httpRespByCall map[int]*http.Response

	sseTargets   []domain.Target
	sseCall      int
	sseErrByCall map[int]error
}

func (p *fakeProxy) ForwardHTTP(_ context.Context, target domain.Target, req *http.Request) (*http.Response, error) {
	p.httpCall++
	p.httpTargets = append(p.httpTargets, target)
	if req != nil && req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		p.httpBodies = append(p.httpBodies, string(raw))
	}
	if err := p.httpErrByCall[p.httpCall]; err != nil {
		return nil, err
	}
	if resp := p.httpRespByCall[p.httpCall]; resp != nil {
		return resp, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func (p *fakeProxy) ForwardSSE(_ context.Context, target domain.Target, _ http.ResponseWriter, _ *http.Request) error {
	p.sseCall++
	p.sseTargets = append(p.sseTargets, target)
	return p.sseErrByCall[p.sseCall]
}

func newTestService(t *testing.T, proxy *fakeProxy, rules domain.Rules, decision domain.Decision, observer Observer) Service {
	t.Helper()
	svc, err := NewService(
		fakeDecisionEngine{decision: decision},
		fakeRulesProvider{rules: rules},
		proxy,
		fakeCompat{},
		fakeStore{},
		fakeQueue{},
		fakeNotifier{},
		fakeLLM{},
		WithObserver(observer),
	)
	if err != nil {
		t.Fatalf("new gateway service: %v", err)
	}
	return svc
}

func TestForwardHTTPWriteFallbackRequiresIdempotencyAndReplay(t *testing.T) {
	t.Parallel()

	proxy := &fakeProxy{
		httpErrByCall: map[int]error{1: errors.New("go upstream down")},
		httpRespByCall: map[int]*http.Response{
			2: {
				StatusCode: http.StatusOK,
				Header:     http.Header{"X-Upstream": []string{"java"}},
				Body:       io.NopCloser(strings.NewReader("fallback-ok")),
			},
		},
	}
	obs := &fakeObserver{}
	svc := newTestService(
		t,
		proxy,
		domain.Rules{
			DefaultTarget:             domain.TargetJava,
			WriteFallbackMethods:      []string{http.MethodPost},
			WriteFallbackPathPrefixes: []string{"/api/messages"},
			IdempotencyHeader:         "Idempotency-Key",
		},
		domain.Decision{Target: domain.TargetGo, Reason: "gray_rule"},
		obs,
	)

	req, _ := http.NewRequest(http.MethodPost, "http://gateway/api/messages/append", strings.NewReader(`{"delta":"hi"}`))
	req.Header.Set("Idempotency-Key", "idem-1")
	resp, decision, err := svc.ForwardHTTP(context.Background(), req)
	if err != nil {
		t.Fatalf("forward http: %v", err)
	}
	defer resp.Body.Close()

	if decision.Target != domain.TargetJava || !strings.Contains(decision.Reason, "fallback_java") {
		t.Fatalf("unexpected fallback decision: %+v", decision)
	}
	if len(proxy.httpTargets) != 2 || proxy.httpTargets[0] != domain.TargetGo || proxy.httpTargets[1] != domain.TargetJava {
		t.Fatalf("unexpected upstream targets: %+v", proxy.httpTargets)
	}
	if len(proxy.httpBodies) != 2 || proxy.httpBodies[0] != `{"delta":"hi"}` || proxy.httpBodies[1] != `{"delta":"hi"}` {
		t.Fatalf("request body replay failed: %+v", proxy.httpBodies)
	}
	if obs.targetHits != 1 || obs.fallbacks != 1 {
		t.Fatalf("unexpected observer stats: hits=%d fallbacks=%d", obs.targetHits, obs.fallbacks)
	}
}

func TestForwardHTTPWriteFallbackWithoutIdempotencyDisabled(t *testing.T) {
	t.Parallel()

	proxy := &fakeProxy{httpErrByCall: map[int]error{1: errors.New("go upstream down")}}
	svc := newTestService(
		t,
		proxy,
		domain.Rules{
			DefaultTarget:             domain.TargetJava,
			WriteFallbackMethods:      []string{http.MethodPost},
			WriteFallbackPathPrefixes: []string{"/api/messages"},
			IdempotencyHeader:         "Idempotency-Key",
		},
		domain.Decision{Target: domain.TargetGo, Reason: "gray_rule"},
		&fakeObserver{},
	)

	req, _ := http.NewRequest(http.MethodPost, "http://gateway/api/messages/append", strings.NewReader(`{"delta":"hi"}`))
	_, _, err := svc.ForwardHTTP(context.Background(), req)
	if err == nil {
		t.Fatal("expected primary proxy error without fallback")
	}
	if proxy.httpCall != 1 {
		t.Fatalf("unexpected fallback call count: %d", proxy.httpCall)
	}
}

func TestForwardSSEFallbackRecordsObserver(t *testing.T) {
	t.Parallel()

	proxy := &fakeProxy{
		sseErrByCall: map[int]error{1: errors.New("go stream closed"), 2: nil},
	}
	obs := &fakeObserver{}
	svc := newTestService(
		t,
		proxy,
		domain.Rules{DefaultTarget: domain.TargetJava},
		domain.Decision{Target: domain.TargetGo, Reason: "gray_rule"},
		obs,
	)

	req, _ := http.NewRequest(http.MethodGet, "http://gateway/sse/chat", nil)
	decision, err := svc.ForwardSSE(context.Background(), httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("forward sse: %v", err)
	}
	if decision.Target != domain.TargetJava {
		t.Fatalf("unexpected sse fallback target: %+v", decision)
	}
	if obs.targetHits != 1 || obs.sseDisconnects != 1 || obs.fallbacks != 1 {
		t.Fatalf("unexpected observer stats: hits=%d disconnects=%d fallbacks=%d", obs.targetHits, obs.sseDisconnects, obs.fallbacks)
	}
}
