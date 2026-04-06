package vector

import (
	"context"
	"errors"
	"testing"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	domain "go-sse-skeleton/internal/domain/vector"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
	port "go-sse-skeleton/internal/port/vector"
)

type fakeEmbedder struct {
	vector []float32
	err    error
}

func (f fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]float32(nil), f.vector...), nil
}

type fakeRepo struct {
	byKB  []domain.Chunk
	allKB []domain.Chunk
	err   error
}

func (f fakeRepo) SimilaritySearchByKB(context.Context, string, string, int) ([]domain.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.Chunk(nil), f.byKB...), nil
}

func (f fakeRepo) SimilaritySearchAllKB(context.Context, string, int) ([]domain.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.Chunk(nil), f.allKB...), nil
}

func (f fakeRepo) KeywordSearchAllKB(context.Context, []string, int) ([]domain.Chunk, error) {
	return nil, nil
}

type noopEventBus struct{}

func (noopEventBus) Publish(context.Context, string, any) error { return nil }

type noopSSE struct{}

func (noopSSE) NotifyDone(context.Context, string) error { return nil }

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

var (
	_ port.EmbeddingProvider = fakeEmbedder{}
	_ port.ChunkRepository   = fakeRepo{}
	_ repo.ChatMessageStore  = nilMessageStore{}
	_ queue.EventPublisher   = noopEventBus{}
	_ sse.MessageNotifier    = noopSSE{}
	_ llm.Client             = noopLLM{}
)

type fakeObserver struct {
	dimMismatch int
	fallbacks   []string
}

func (o *fakeObserver) RecordFallback(scope string, reason string) {
	o.fallbacks = append(o.fallbacks, scope+":"+reason)
}

func (o *fakeObserver) RecordDimensionMismatch(_ int, _ int) {
	o.dimMismatch++
}

func TestSimilaritySearchByKBThresholdBoundaryAndDimValidation(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{}
	svc, err := NewService(
		fakeEmbedder{vector: []float32{1, 0}},
		fakeRepo{
			byKB: []domain.Chunk{
				{ID: "c1", KBID: "kb1", Content: "keep-0.60", Embedding: []float32{0.6, 0.8}},      // exactly 0.60
				{ID: "c2", KBID: "kb1", Content: "drop-0.50", Embedding: []float32{0.5, 0.8660254}}, // below threshold
				{ID: "c3", KBID: "kb1", Content: "drop-dim", Embedding: []float32{1, 0, 0}},         // dim mismatch
			},
		},
		nilMessageStore{},
		noopEventBus{},
		noopSSE{},
		noopLLM{},
		Config{MinConfidence: 0.60, LimitByKB: 3},
		WithObserver(observer),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	out, err := svc.SimilaritySearch(context.Background(), "kb1", "query", 3)
	if err != nil {
		t.Fatalf("similarity search by kb: %v", err)
	}
	if len(out) != 1 || out[0] != "keep-0.60" {
		t.Fatalf("unexpected result: %+v", out)
	}
	if observer.dimMismatch != 1 {
		t.Fatalf("expected one dimension mismatch record, got %d", observer.dimMismatch)
	}
}

func TestSimilaritySearchAllKB(t *testing.T) {
	t.Parallel()

	svc, err := NewService(
		fakeEmbedder{vector: []float32{1, 0}},
		fakeRepo{allKB: []domain.Chunk{{Content: "global-hit", Embedding: []float32{1, 0}}}},
		nilMessageStore{},
		noopEventBus{},
		noopSSE{},
		noopLLM{},
		Config{MinConfidence: 0.60, LimitAllKB: 5},
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	out, err := svc.SimilaritySearchAllKnowledgeBases(context.Background(), "query", 5)
	if err != nil {
		t.Fatalf("similarity search all kb: %v", err)
	}
	if len(out) != 1 || out[0] != "global-hit" {
		t.Fatalf("unexpected all-kb result: %+v", out)
	}
}

func TestSimilaritySearchEmbedFailureDegradesToEmpty(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{}
	svc, err := NewService(
		fakeEmbedder{err: errors.New("embed failed")},
		fakeRepo{},
		nilMessageStore{},
		noopEventBus{},
		noopSSE{},
		noopLLM{},
		Config{MinConfidence: 0.60},
		WithObserver(observer),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	out, err := svc.SimilaritySearch(context.Background(), "kb1", "query", 3)
	if err != nil {
		t.Fatalf("expected graceful downgrade, got err=%v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result on embed failure, got %+v", out)
	}
	if len(observer.fallbacks) == 0 {
		t.Fatal("expected fallback observer record")
	}
}

func TestSimilaritySearchRepoFailureDegradesToEmpty(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{}
	svc, err := NewService(
		fakeEmbedder{vector: []float32{1, 0}},
		fakeRepo{err: errors.New("db failed")},
		nilMessageStore{},
		noopEventBus{},
		noopSSE{},
		noopLLM{},
		Config{MinConfidence: 0.60},
		WithObserver(observer),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	out, err := svc.SimilaritySearchAllKnowledgeBases(context.Background(), "query", 5)
	if err != nil {
		t.Fatalf("expected graceful downgrade, got err=%v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result on db failure, got %+v", out)
	}
	if len(observer.fallbacks) == 0 {
		t.Fatal("expected fallback observer record")
	}
}

type nilMessageStore struct{}

func (nilMessageStore) Create(context.Context, *chatdomain.Message) (string, error) { return "", nil }

func (nilMessageStore) GetByID(context.Context, string) (*chatdomain.Message, error) {
	return nil, nil
}

func (nilMessageStore) ListBySession(context.Context, string) ([]*chatdomain.Message, error) {
	return nil, nil
}

func (nilMessageStore) ListRecentBySession(context.Context, string, int) ([]*chatdomain.Message, error) {
	return nil, nil
}

func (nilMessageStore) Update(context.Context, *chatdomain.Message) error { return nil }

func (nilMessageStore) Delete(context.Context, string) error { return nil }
