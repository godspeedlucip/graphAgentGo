package memory

import (
	"context"
	"errors"
	"log/slog"
	"time"

	domain "go-sse-skeleton/internal/domain/memory"
	port "go-sse-skeleton/internal/port/memory"
)

type Service interface {
	Add(ctx context.Context, conversationID string, messages []domain.Message) error
	Get(ctx context.Context, conversationID string) (GetResult, error)
	Clear(ctx context.Context, conversationID string) error
}

type service struct {
	cacheStore      port.CacheStore
	messageReader   port.MessageReader
	codec           port.Codec
	mqPublisher     port.MQPublisher
	websocketPusher port.WebsocketPusher
	paymentGateway  port.PaymentGateway
	txManager       port.TxManager
	observer        Observer
	cfg             Config
}

func NewService(
	cacheStore port.CacheStore,
	messageReader port.MessageReader,
	codec port.Codec,
	mqPublisher port.MQPublisher,
	websocketPusher port.WebsocketPusher,
	paymentGateway port.PaymentGateway,
	txManager port.TxManager,
	cfg Config,
	opts ...Option,
) (Service, error) {
	if cacheStore == nil || messageReader == nil || codec == nil || mqPublisher == nil || websocketPusher == nil || paymentGateway == nil || txManager == nil {
		return nil, errors.New("nil dependency in memory service")
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 30
	}
	if cfg.TTLHours <= 0 {
		cfg.TTLHours = 24
	}
	svc := &service{
		cacheStore:      cacheStore,
		messageReader:   messageReader,
		codec:           codec,
		mqPublisher:     mqPublisher,
		websocketPusher: websocketPusher,
		paymentGateway:  paymentGateway,
		txManager:       txManager,
		observer:        NewNoopObserver(),
		cfg:             cfg,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if svc.observer == nil {
		svc.observer = NewNoopObserver()
	}
	return svc, nil
}

// TODO: wire mqPublisher/websocketPusher/paymentGateway when memory domain emits async events
// (e.g., cache rebuild notification, usage metering). Kept injected now to preserve architecture contracts.
// Tx boundary notes:
// 1) DB fallback read is now executed inside txManager.WithinTx in Get().
// 2) Cache backfill (Add/ReplaceWindow) stays outside DB transaction on purpose to avoid holding DB tx on Redis I/O.
// TODO: if future flow needs strict "DB snapshot + cache refresh" atomicity, introduce outbox/version gating around Add().

func (s *service) Add(ctx context.Context, conversationID string, messages []domain.Message) error {
	if conversationID == "" {
		return domain.ErrInvalidInput
	}

	if len(messages) > s.cfg.MaxMessages {
		messages = messages[len(messages)-s.cfg.MaxMessages:]
	}

	cached, err := s.codec.RuntimeToCached(messages)
	if err != nil {
		return err
	}
	payloads, err := s.codec.EncodeCached(cached)
	if err != nil {
		return err
	}

	key := domain.CacheKey(conversationID)
	if err = s.cacheStore.ReplaceWindow(ctx, key, payloads, time.Duration(s.cfg.TTLHours)*time.Hour); err != nil {
		// Keep Java behavior in DistributedChatMemory.add:
		// cache write failures should not block main workflow.
		slog.Warn("memory cache write skipped", "conversationID", conversationID, "err", err)
		return nil
	}
	return nil
}

func (s *service) Get(ctx context.Context, conversationID string) (GetResult, error) {
	if conversationID == "" {
		return GetResult{}, domain.ErrInvalidInput
	}
	key := domain.CacheKey(conversationID)

	payloads, err := s.cacheStore.Range(ctx, key, int64(-s.cfg.MaxMessages), -1)
	if err == nil && len(payloads) > 0 {
		cached, decErr := s.codec.DecodeCached(payloads)
		if decErr == nil {
			msgs, convErr := s.codec.CachedToRuntime(cached)
			if convErr == nil {
				s.observer.RecordGetSource("cache")
				return GetResult{Messages: msgs, Source: "cache"}, nil
			}
		}
		// Keep Java behavior: corrupted cache should be evicted, then fallback to DB.
		s.observer.RecordFallback("cache_corrupted")
		_ = s.cacheStore.Delete(ctx, key)
	} else if err != nil {
		// Keep Java behavior in DistributedChatMemory.get:
		// cache read failure should fallback to DB without aborting request.
		slog.Warn("memory cache read skipped", "conversationID", conversationID, "err", err)
		s.observer.RecordFallback("cache_read_failed")
	}
	var (
		dbMessages []domain.Message
		dbErr      error
	)
	if txErr := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		dbMessages, dbErr = s.messageReader.ListRecentBySession(txCtx, conversationID, s.cfg.MaxMessages)
		return dbErr
	}); txErr != nil {
		return GetResult{}, txErr
	}
	if dbErr != nil {
		return GetResult{}, dbErr
	}
	normalized, normErr := s.normalizeMessages(dbMessages)
	if normErr == nil {
		dbMessages = normalized
	} else {
		slog.Warn("memory db messages normalize skipped", "conversationID", conversationID, "err", normErr)
		s.observer.RecordFallback("normalize_failed")
	}
	s.observer.RecordGetSource("db_fallback")

	if addErr := s.Add(ctx, conversationID, dbMessages); addErr != nil {
		slog.Warn("memory backfill failed", "conversationID", conversationID, "err", addErr)
	}
	return GetResult{Messages: dbMessages, Source: "db_fallback"}, nil
}

func (s *service) normalizeMessages(messages []domain.Message) ([]domain.Message, error) {
	cached, err := s.codec.RuntimeToCached(messages)
	if err != nil {
		return nil, err
	}
	return s.codec.CachedToRuntime(cached)
}

func (s *service) Clear(ctx context.Context, conversationID string) error {
	if conversationID == "" {
		return domain.ErrInvalidInput
	}
	if err := s.cacheStore.Delete(ctx, domain.CacheKey(conversationID)); err != nil {
		// Keep Java behavior in DistributedChatMemory.clear:
		// cache clear failure should not break caller flow.
		slog.Warn("memory cache clear skipped", "conversationID", conversationID, "err", err)
		return nil
	}
	return nil
}
