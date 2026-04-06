package vector

import (
	"context"

	domain "go-sse-skeleton/internal/domain/vector"
)

type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type ChunkRepository interface {
	SimilaritySearchByKB(ctx context.Context, kbID string, vectorLiteral string, limit int) ([]domain.Chunk, error)
	SimilaritySearchAllKB(ctx context.Context, vectorLiteral string, limit int) ([]domain.Chunk, error)
	KeywordSearchAllKB(ctx context.Context, keywords []string, limit int) ([]domain.Chunk, error)
}