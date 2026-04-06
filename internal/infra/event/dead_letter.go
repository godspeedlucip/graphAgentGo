package event

import (
	"context"
	"log/slog"
	"sync"

	domain "go-sse-skeleton/internal/domain/event"
)

type NoopDeadLetterSink struct{}

func NewNoopDeadLetterSink() *NoopDeadLetterSink { return &NoopDeadLetterSink{} }

func (s *NoopDeadLetterSink) Push(_ context.Context, evt domain.ChatEvent, reason string, attempts int) error {
	slog.Error("event pushed to dlq", "eventID", evt.EventID, "agentID", evt.AgentID, "sessionID", evt.SessionID, "attempts", attempts, "reason", reason)
	return nil
}

type DeadLetterRecord struct {
	Event    domain.ChatEvent
	Reason   string
	Attempts int
}

type InMemoryDeadLetterSink struct {
	mu      sync.Mutex
	records []DeadLetterRecord
}

func NewInMemoryDeadLetterSink() *InMemoryDeadLetterSink {
	return &InMemoryDeadLetterSink{}
}

func (s *InMemoryDeadLetterSink) Push(_ context.Context, evt domain.ChatEvent, reason string, attempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, DeadLetterRecord{
		Event:    evt,
		Reason:   reason,
		Attempts: attempts,
	})
	return nil
}

func (s *InMemoryDeadLetterSink) Records() []DeadLetterRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DeadLetterRecord, len(s.records))
	copy(out, s.records)
	return out
}
