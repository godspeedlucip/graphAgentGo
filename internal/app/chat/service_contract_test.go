package chat

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/chat"
	"go-sse-skeleton/internal/port/cache"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	"go-sse-skeleton/internal/port/repository"
	sseport "go-sse-skeleton/internal/port/sse"
)

var _ repository.ChatMessageStore = (*fakeStore)(nil)
var _ repository.ChatMessageAppender = (*fakeStore)(nil)

type fakeStore struct {
	mu       sync.Mutex
	seq      int
	messages map[string]*domain.Message
}

func newFakeStore() *fakeStore {
	return &fakeStore{messages: make(map[string]*domain.Message)}
}

func (s *fakeStore) Create(_ context.Context, msg *domain.Message) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := "m-" + strconv.Itoa(s.seq)
	cp := *msg
	cp.ID = id
	now := time.Now().Add(time.Duration(s.seq) * time.Millisecond)
	cp.CreatedAt = now
	cp.UpdatedAt = now
	s.messages[id] = &cp
	return id, nil
}

func (s *fakeStore) GetByID(_ context.Context, id string) (*domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg, ok := s.messages[id]
	if !ok {
		return nil, nil
	}
	cp := *msg
	return &cp, nil
}

func (s *fakeStore) ListBySession(_ context.Context, sessionID string) ([]*domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Message, 0)
	for _, m := range s.messages {
		if m.SessionID == sessionID {
			cp := *m
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *fakeStore) ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error) {
	all, err := s.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(all) <= limit {
		return all, nil
	}
	return all[len(all)-limit:], nil
}

func (s *fakeStore) Update(_ context.Context, msg *domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.messages[msg.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *msg
	cp.UpdatedAt = time.Now()
	s.messages[msg.ID] = &cp
	return nil
}

func (s *fakeStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.messages[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.messages, id)
	return nil
}

func (s *fakeStore) AppendContent(_ context.Context, id string, delta string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg, ok := s.messages[id]
	if !ok {
		return domain.ErrNotFound
	}
	msg.Content += delta
	msg.UpdatedAt = time.Now()
	return nil
}

type noopCache struct{}

func (noopCache) Invalidate(context.Context, string) error { return nil }

var _ cache.ChatMemoryCache = noopCache{}

type noopPublisher struct{}

func (noopPublisher) PublishChatEvent(context.Context, string, string, string) error { return nil }

var _ queue.ChatEventPublisher = noopPublisher{}

type noopSSE struct{}

func (noopSSE) NotifyDone(context.Context, string) error { return nil }

var _ sseport.MessageNotifier = noopSSE{}

type noopLLM struct{}

func (noopLLM) Generate(context.Context, string) (string, error) { return "", nil }

var _ llm.Client = noopLLM{}

func TestServiceCreateAppendRecentAndMetadata(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	svc, err := NewService(store, noopCache{}, noopPublisher{}, noopSSE{}, noopLLM{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	id1, err := svc.CreateFromCommand(context.Background(), CreateMessageCommand{
		SessionID: "s1",
		Role:      domain.RoleAssistant,
		Content:   "hello",
		Metadata: &domain.Metadata{
			ToolCalls: []any{"call-1"},
		},
	})
	if err != nil {
		t.Fatalf("create message1: %v", err)
	}

	id2, err := svc.CreateFromCommand(context.Background(), CreateMessageCommand{
		SessionID: "s1",
		Role:      domain.RoleAssistant,
		Content:   "world",
		Metadata: &domain.Metadata{
			ToolResponse: map[string]any{"k": "v"},
		},
	})
	if err != nil {
		t.Fatalf("create message2: %v", err)
	}

	if err = svc.Append(context.Background(), id1, " delta"); err != nil {
		t.Fatalf("append: %v", err)
	}

	msg, err := store.GetByID(context.Background(), id1)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if msg.Content != "hello delta" {
		t.Fatalf("unexpected appended content: %q", msg.Content)
	}
	if msg.Metadata == nil || len(msg.Metadata.ToolCalls) != 1 || msg.Metadata.ToolCalls[0] != "call-1" {
		t.Fatalf("metadata lost after append: %+v", msg.Metadata)
	}

	recent, err := svc.ListRecentBySession(context.Background(), "s1", 2)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent messages, got %d", len(recent))
	}
	if recent[0].ID != id1 || recent[1].ID != id2 {
		t.Fatalf("recent order mismatch: got [%s,%s], want [%s,%s]", recent[0].ID, recent[1].ID, id1, id2)
	}
}
