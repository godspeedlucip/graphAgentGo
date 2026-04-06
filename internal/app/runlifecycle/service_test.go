package runlifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/runlifecycle"
	infrrl "go-sse-skeleton/internal/infra/runlifecycle"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	port "go-sse-skeleton/internal/port/runlifecycle"
	"go-sse-skeleton/internal/port/sse"
)

type fakeRuntime struct {
	runFn func(ctx context.Context, in port.RuntimeInput) (port.RuntimeOutput, error)
}

func (r fakeRuntime) Run(ctx context.Context, in port.RuntimeInput) (port.RuntimeOutput, error) {
	return r.runFn(ctx, in)
}

type fakeRuntimeProvider struct {
	runtime port.Runtime
	err     error
}

func (p fakeRuntimeProvider) GetRuntime(context.Context, string, string) (port.Runtime, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.runtime, nil
}

type fakePublisher struct {
	mu  sync.Mutex
	err error
}

func (p *fakePublisher) PublishLifecycle(context.Context, domain.LifecycleEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

type fakeNotifier struct {
	mu          sync.Mutex
	deltas      int
	errOnDelta  error
	errOnOthers error
}

func (n *fakeNotifier) NotifyStarted(context.Context, string, string) error { return n.errOnOthers }

func (n *fakeNotifier) NotifyDelta(context.Context, string, string, string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.deltas++
	return n.errOnDelta
}

func (n *fakeNotifier) NotifyDone(context.Context, string, string) error             { return n.errOnOthers }
func (n *fakeNotifier) NotifyFailed(context.Context, string, string, string) error    { return n.errOnOthers }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type noopQueue struct{}

func (noopQueue) Publish(context.Context, string, any) error { return nil }

type noopSSE struct{}

func (noopSSE) NotifyDone(context.Context, string) error { return nil }

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

type testChatStore struct{}

func (testChatStore) Create(context.Context, *chatdomain.Message) (string, error) { return "", nil }
func (testChatStore) GetByID(context.Context, string) (*chatdomain.Message, error) { return nil, nil }
func (testChatStore) ListBySession(context.Context, string) ([]*chatdomain.Message, error) {
	return nil, nil
}
func (testChatStore) ListRecentBySession(context.Context, string, int) ([]*chatdomain.Message, error) {
	return nil, nil
}
func (testChatStore) Update(context.Context, *chatdomain.Message) error { return nil }
func (testChatStore) Delete(context.Context, string) error              { return nil }

var _ llm.Client = noopLLM{}
var _ queue.EventPublisher = noopQueue{}
var _ sse.MessageNotifier = noopSSE{}
var _ repo.ChatMessageStore = testChatStore{}

func newServiceForTest(t *testing.T, runtime port.Runtime, notifier *fakeNotifier, publisher *fakePublisher, opts ...Option) Service {
	t.Helper()
	repoImpl := infrrl.NewInMemoryRunRepository()
	guard := infrrl.NewInMemoryGuard()
	orchestrator, err := NewOrchestrator(repoImpl, publisher, notifier, fixedClock{t: time.Unix(1710000000, 0)})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	svc, err := NewService(
		fakeRuntimeProvider{runtime: runtime},
		repoImpl,
		guard,
		orchestrator,
		fixedClock{t: time.Unix(1710000000, 0)},
		testChatStore{},
		noopQueue{},
		noopSSE{},
		noopLLM{},
		opts...,
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func TestRunStartIdempotencyConflict(t *testing.T) {
	t.Parallel()

	svc := newServiceForTest(t, fakeRuntime{runFn: func(context.Context, port.RuntimeInput) (port.RuntimeOutput, error) {
		return port.RuntimeOutput{Text: "ok"}, nil
	}}, &fakeNotifier{}, &fakePublisher{})

	if _, err := svc.Start(context.Background(), StartRunCommand{RunID: "r1", AgentID: "a1", SessionID: "s1"}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	_, err := svc.Start(context.Background(), StartRunCommand{RunID: "r1", AgentID: "a1", SessionID: "s1"})
	if !errors.Is(err, domain.ErrRunAlreadyExists) {
		t.Fatalf("expected run already exists, got %v", err)
	}
}

func TestRunTimeoutStatus(t *testing.T) {
	t.Parallel()

	svc := newServiceForTest(t, fakeRuntime{runFn: func(ctx context.Context, _ port.RuntimeInput) (port.RuntimeOutput, error) {
		<-ctx.Done()
		return port.RuntimeOutput{}, ctx.Err()
	}}, &fakeNotifier{}, &fakePublisher{})

	result, err := svc.Start(context.Background(), StartRunCommand{
		RunID: "r-timeout", AgentID: "a1", SessionID: "s1", Timeout: 10 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.Status != domain.StatusTimeout {
		t.Fatalf("expected timeout status, got %s", result.Status)
	}
}

func TestRunCancelPropagatesToRuntime(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	svc := newServiceForTest(t, fakeRuntime{runFn: func(ctx context.Context, _ port.RuntimeInput) (port.RuntimeOutput, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return port.RuntimeOutput{}, ctx.Err()
	}}, &fakeNotifier{}, &fakePublisher{})

	done := make(chan RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := svc.Start(context.Background(), StartRunCommand{RunID: "r-cancel", AgentID: "a1", SessionID: "s1"})
		done <- res
		errCh <- err
	}()
	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("runtime not started")
	}
	if err := svc.Cancel(context.Background(), CancelRunCommand{RunID: "r-cancel", Cause: "user"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled error, got %v", err)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timeout waiting canceled run")
	}
	res := <-done
	if res.Status != domain.StatusCanceled {
		t.Fatalf("expected canceled status, got %s", res.Status)
	}
}

func TestRunDeltaAndSideChannelFailureDoesNotBreakMainResult(t *testing.T) {
	t.Parallel()

	notifier := &fakeNotifier{errOnDelta: errors.New("sse delta down"), errOnOthers: errors.New("sse down")}
	publisher := &fakePublisher{err: errors.New("event down")}
	svc := newServiceForTest(t, fakeRuntime{runFn: func(ctx context.Context, in port.RuntimeInput) (port.RuntimeOutput, error) {
		_ = in.AppendOutput(ctx, "A")
		_ = in.AppendOutput(ctx, "B")
		return port.RuntimeOutput{}, nil
	}}, notifier, publisher)

	res, err := svc.Start(context.Background(), StartRunCommand{RunID: "r-delta", AgentID: "a1", SessionID: "s1"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if res.Status != domain.StatusDone || res.Output != "AB" {
		t.Fatalf("unexpected result: %+v", res)
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if notifier.deltas < 2 {
		t.Fatalf("expected delta notifications >=2, got %d", notifier.deltas)
	}
}
