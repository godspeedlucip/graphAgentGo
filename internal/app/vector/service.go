package vector

import (
	"context"
	"errors"
	"log/slog"

	domain "go-sse-skeleton/internal/domain/vector"
	"go-sse-skeleton/internal/port/llm"
	"go-sse-skeleton/internal/port/queue"
	repo "go-sse-skeleton/internal/port/repository"
	"go-sse-skeleton/internal/port/sse"
	port "go-sse-skeleton/internal/port/vector"
)

type Service interface {
	SimilaritySearch(ctx context.Context, kbID string, query string, limit int) ([]string, error)
	SimilaritySearchAllKnowledgeBases(ctx context.Context, query string, limit int) ([]string, error)
}

type service struct {
	embeddingProvider port.EmbeddingProvider
	chunkRepository   port.ChunkRepository

	// Shared dependencies are injected to keep architecture contracts consistent.
	messageStore repo.ChatMessageStore
	eventBus     queue.EventPublisher
	sseNotifier  sse.MessageNotifier
	llmClient    llm.Client

	cfg Config
}

func NewService(
	embeddingProvider port.EmbeddingProvider,
	chunkRepository port.ChunkRepository,
	messageStore repo.ChatMessageStore,
	eventBus queue.EventPublisher,
	sseNotifier sse.MessageNotifier,
	llmClient llm.Client,
	cfg Config,
) (Service, error) {
	if embeddingProvider == nil || chunkRepository == nil || messageStore == nil || eventBus == nil || sseNotifier == nil || llmClient == nil {
		return nil, errors.New("nil dependency in vector service")
	}
	if cfg.ModelName == "" {
		cfg.ModelName = "bge-m3"
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.60
	}
	if cfg.LimitByKB <= 0 {
		cfg.LimitByKB = 3
	}
	if cfg.LimitAllKB <= 0 {
		cfg.LimitAllKB = 5
	}
	return &service{
		embeddingProvider: embeddingProvider,
		chunkRepository:   chunkRepository,
		messageStore:      messageStore,
		eventBus:          eventBus,
		sseNotifier:       sseNotifier,
		llmClient:         llmClient,
		cfg:               cfg,
	}, nil
}

func (s *service) SimilaritySearch(ctx context.Context, kbID string, query string, limit int) ([]string, error) {
	if kbID == "" || query == "" {
		return nil, domain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = s.cfg.LimitByKB
	}

	vector, err := s.embeddingProvider.Embed(ctx, query)
	if err != nil {
		// Keep Java behavior: fail-open by returning empty results for retrieval failures.
		slog.Warn("vector embed failed", "kbID", kbID, "err", err)
		return []string{}, nil
	}

	vectorLiteral := domain.ToVectorLiteral(vector)
	chunks, err := s.chunkRepository.SimilaritySearchByKB(ctx, kbID, vectorLiteral, limit)
	if err != nil {
		slog.Warn("vector similarity search by kb failed", "kbID", kbID, "err", err)
		return []string{}, nil
	}

	filtered := s.filterByConfidence(chunks, vector)
	return toContents(filtered), nil
}

func (s *service) SimilaritySearchAllKnowledgeBases(ctx context.Context, query string, limit int) ([]string, error) {
	if query == "" {
		return nil, domain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = s.cfg.LimitAllKB
	}

	vector, err := s.embeddingProvider.Embed(ctx, query)
	if err != nil {
		slog.Warn("vector embed failed", "err", err)
		return []string{}, nil
	}

	vectorLiteral := domain.ToVectorLiteral(vector)
	chunks, err := s.chunkRepository.SimilaritySearchAllKB(ctx, vectorLiteral, limit)
	if err != nil {
		slog.Warn("vector similarity search all kb failed", "err", err)
		return []string{}, nil
	}

	filtered := s.filterByConfidence(chunks, vector)
	return toContents(filtered), nil
}

func toContents(chunks []domain.Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.Content == "" {
			continue
		}
		out = append(out, chunk.Content)
	}
	return out
}

func (s *service) filterByConfidence(chunks []domain.Chunk, queryVector []float32) []domain.Chunk {
	if len(chunks) == 0 || len(queryVector) == 0 {
		return []domain.Chunk{}
	}

	out := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.Content == "" || len(chunk.Embedding) == 0 {
			continue
		}
		score := domain.CosineSimilarity(queryVector, chunk.Embedding)
		if score >= s.cfg.MinConfidence {
			out = append(out, chunk)
		}
	}
	return out
}
