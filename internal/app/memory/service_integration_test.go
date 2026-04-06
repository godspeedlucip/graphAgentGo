package memory

import (
	"context"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/memory"
	inframemory "go-sse-skeleton/internal/infra/memory"
	port "go-sse-skeleton/internal/port/memory"
)

type fakeCacheStore struct {
	data        map[string][]string
	deleteCalls int
}

func newFakeCacheStore() *fakeCacheStore {
	return &fakeCacheStore{data: make(map[string][]string)}
}

func (c *fakeCacheStore) Range(_ context.Context, key string, _ int64, _ int64) ([]string, error) {
	return append([]string(nil), c.data[key]...), nil
}

func (c *fakeCacheStore) ReplaceWindow(_ context.Context, key string, payloads []string, _ time.Duration) error {
	c.data[key] = append([]string(nil), payloads...)
	return nil
}

func (c *fakeCacheStore) Delete(_ context.Context, key string) error {
	c.deleteCalls++
	delete(c.data, key)
	return nil
}

type fakeMessageReader struct {
	calls    int
	messages []domain.Message
}

func (r *fakeMessageReader) ListRecentBySession(_ context.Context, _ string, _ int) ([]domain.Message, error) {
	r.calls++
	return append([]domain.Message(nil), r.messages...), nil
}

type noopMQ struct{}

func (noopMQ) Publish(context.Context, string, any) error { return nil }

type noopWS struct{}

func (noopWS) Push(context.Context, string, any) error { return nil }

type noopPay struct{}

func (noopPay) Charge(context.Context, string, int64, string) (string, error) { return "", nil }

type fakeTxManager struct {
	calls int
}

func (m *fakeTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.calls++
	return fn(ctx)
}

var _ port.TxManager = (*fakeTxManager)(nil)

func TestMemoryGetCacheHit(t *testing.T) {
	t.Parallel()

	cache := newFakeCacheStore()
	codec := inframemory.NewJSONCodec()
	cachedPayloads, err := codec.EncodeCached([]domain.CachedMessage{
		{
			Role:    domain.RoleUser,
			Content: "hello",
		},
	})
	if err != nil {
		t.Fatalf("encode cached payload: %v", err)
	}
	cache.data[domain.CacheKey("c1")] = cachedPayloads

	reader := &fakeMessageReader{}
	txm := &fakeTxManager{}
	svc, err := NewService(cache, reader, codec, noopMQ{}, noopWS{}, noopPay{}, txm, Config{MaxMessages: 30, TTLHours: 24})
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}

	got, err := svc.Get(context.Background(), "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Source != "cache" {
		t.Fatalf("expected source cache, got %s", got.Source)
	}
	if reader.calls != 0 {
		t.Fatalf("expected db reader not called on cache hit, got %d", reader.calls)
	}
	if txm.calls != 0 {
		t.Fatalf("expected tx manager not called on cache hit, got %d", txm.calls)
	}
}

func TestMemoryGetCorruptedCacheFallbackToDB(t *testing.T) {
	t.Parallel()

	cache := newFakeCacheStore()
	cache.data[domain.CacheKey("c2")] = []string{"{broken-json"}

	reader := &fakeMessageReader{
		messages: []domain.Message{
			{Role: domain.RoleUser, Content: "from-db"},
		},
	}
	txm := &fakeTxManager{}
	svc, err := NewService(cache, reader, inframemory.NewJSONCodec(), noopMQ{}, noopWS{}, noopPay{}, txm, Config{MaxMessages: 30, TTLHours: 24})
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}

	got, err := svc.Get(context.Background(), "c2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Source != "db_fallback" {
		t.Fatalf("expected source db_fallback, got %s", got.Source)
	}
	if reader.calls != 1 {
		t.Fatalf("expected db reader called once, got %d", reader.calls)
	}
	if txm.calls != 1 {
		t.Fatalf("expected tx manager called once for db fallback, got %d", txm.calls)
	}
	if cache.deleteCalls == 0 {
		t.Fatalf("expected corrupted cache eviction")
	}
	if len(cache.data[domain.CacheKey("c2")]) == 0 {
		t.Fatalf("expected db fallback backfill cache payloads")
	}
}

func TestMemoryGetFiltersOrphanToolFromDBFallback(t *testing.T) {
	t.Parallel()

	cache := newFakeCacheStore()
	reader := &fakeMessageReader{
		messages: []domain.Message{
			{
				Role:    domain.RoleTool,
				Content: "orphan",
				Metadata: &domain.Metadata{
					ToolResponse: &domain.ToolResponse{
						ID:           "t1",
						Name:         "tool",
						ResponseData: "x",
					},
				},
			},
			{
				Role:    domain.RoleAssistant,
				Content: "ok",
			},
		},
	}
	svc, err := NewService(cache, reader, inframemory.NewJSONCodec(), noopMQ{}, noopWS{}, noopPay{}, &fakeTxManager{}, Config{MaxMessages: 30, TTLHours: 24})
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}

	got, err := svc.Get(context.Background(), "c3")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected orphan tool filtered, got %d messages", len(got.Messages))
	}
	if got.Messages[0].Role != domain.RoleAssistant {
		t.Fatalf("expected assistant message after filtering, got %s", got.Messages[0].Role)
	}
}
