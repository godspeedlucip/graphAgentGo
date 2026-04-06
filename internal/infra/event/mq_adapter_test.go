package event

import (
	"context"
	"testing"

	domain "go-sse-skeleton/internal/domain/event"
)

type captureHandler struct {
	events []domain.ChatEvent
}

func (h *captureHandler) Handle(_ context.Context, evt domain.ChatEvent) error {
	h.events = append(h.events, evt)
	return nil
}

func TestBrokerPublisherSubscriberRoundTrip(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryBroker()
	sub, err := NewBrokerSubscriber(broker, DefaultChatEventTopic)
	if err != nil {
		t.Fatalf("new subscriber: %v", err)
	}
	h := &captureHandler{}
	if err = sub.SubscribeChatEvent(context.Background(), h); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	pub, err := NewBrokerPublisher(broker, DefaultChatEventTopic)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	evt := domain.ChatEvent{EventID: "evt-1", AgentID: "a1", SessionID: "s1", UserInput: "hello"}
	if err = pub.PublishChatEvent(context.Background(), evt); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(h.events) != 1 {
		t.Fatalf("expected one consumed event, got %d", len(h.events))
	}
	if h.events[0].EventID != evt.EventID || h.events[0].AgentID != evt.AgentID || h.events[0].SessionID != evt.SessionID {
		t.Fatalf("unexpected consumed event: %+v", h.events[0])
	}
}
