package event

import (
	"context"
	"encoding/json"
	"errors"

	domain "go-sse-skeleton/internal/domain/event"
	port "go-sse-skeleton/internal/port/event"
)

const DefaultChatEventTopic = "chat.event"

type BrokerPublisher struct {
	broker port.MessageBroker
	topic  string
}

func NewBrokerPublisher(broker port.MessageBroker, topic string) (*BrokerPublisher, error) {
	if broker == nil {
		return nil, errors.New("nil broker")
	}
	if topic == "" {
		topic = DefaultChatEventTopic
	}
	return &BrokerPublisher{broker: broker, topic: topic}, nil
}

func (p *BrokerPublisher) PublishChatEvent(ctx context.Context, evt domain.ChatEvent) error {
	if err := evt.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return p.broker.Publish(ctx, port.BrokerMessage{
		Topic:   p.topic,
		Key:     evt.EventID,
		Payload: payload,
	})
}

type BrokerSubscriber struct {
	broker port.MessageBroker
	topic  string
}

func NewBrokerSubscriber(broker port.MessageBroker, topic string) (*BrokerSubscriber, error) {
	if broker == nil {
		return nil, errors.New("nil broker")
	}
	if topic == "" {
		topic = DefaultChatEventTopic
	}
	return &BrokerSubscriber{broker: broker, topic: topic}, nil
}

func (s *BrokerSubscriber) SubscribeChatEvent(ctx context.Context, handler port.ChatEventHandler) error {
	if handler == nil {
		return errors.New("nil chat event handler")
	}
	return s.broker.Subscribe(ctx, s.topic, func(msgCtx context.Context, msg port.BrokerMessage) error {
		var evt domain.ChatEvent
		if err := json.Unmarshal(msg.Payload, &evt); err != nil {
			return err
		}
		return handler.Handle(msgCtx, evt)
	})
}

var _ port.Publisher = (*BrokerPublisher)(nil)
var _ port.Subscriber = (*BrokerSubscriber)(nil)
