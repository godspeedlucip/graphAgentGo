package vector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	domain "go-sse-skeleton/internal/domain/vector"
)

type PGVectorRepository struct {
	db *sql.DB
}

func NewPGVectorRepository(db *sql.DB) (*PGVectorRepository, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}
	return &PGVectorRepository{db: db}, nil
}

func (r *PGVectorRepository) SimilaritySearchByKB(ctx context.Context, kbID string, vectorLiteral string, limit int) ([]domain.Chunk, error) {
	if kbID == "" || vectorLiteral == "" || limit <= 0 {
		return nil, domain.ErrInvalidInput
	}
	const q = `
SELECT
    id::text,
    kb_id::text,
    doc_id::text,
    content,
    metadata::text,
    embedding::text
FROM chunk_bge_m3
WHERE kb_id::text = $1
ORDER BY embedding <-> CAST($2 AS vector)
LIMIT $3
`
	rows, err := r.db.QueryContext(ctx, q, kbID, vectorLiteral, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *PGVectorRepository) SimilaritySearchAllKB(ctx context.Context, vectorLiteral string, limit int) ([]domain.Chunk, error) {
	if vectorLiteral == "" || limit <= 0 {
		return nil, domain.ErrInvalidInput
	}
	const q = `
SELECT
    id::text,
    kb_id::text,
    doc_id::text,
    content,
    metadata::text,
    embedding::text
FROM chunk_bge_m3
ORDER BY embedding <-> CAST($1 AS vector)
LIMIT $2
`
	rows, err := r.db.QueryContext(ctx, q, vectorLiteral, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *PGVectorRepository) KeywordSearchAllKB(ctx context.Context, keywords []string, limit int) ([]domain.Chunk, error) {
	if limit <= 0 {
		return nil, domain.ErrInvalidInput
	}
	// TODO: align ranking formula with Java mapper's CASE-sum weighting if keyword search becomes primary path.
	if len(keywords) == 0 {
		const q = `
SELECT
    id::text,
    kb_id::text,
    doc_id::text,
    content,
    metadata::text,
    embedding::text
FROM chunk_bge_m3
ORDER BY updated_at DESC
LIMIT $1
`
		rows, err := r.db.QueryContext(ctx, q, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanChunks(rows)
	}

	parts := make([]string, 0, len(keywords))
	args := make([]any, 0, len(keywords)+1)
	argIdx := 1
	for _, kw := range keywords {
		if strings.TrimSpace(kw) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("lower(content) LIKE lower($%d)", argIdx))
		args = append(args, "%"+kw+"%")
		argIdx++
	}
	if len(parts) == 0 {
		return []domain.Chunk{}, nil
	}
	args = append(args, limit)

	q := fmt.Sprintf(`
SELECT
    id::text,
    kb_id::text,
    doc_id::text,
    content,
    metadata::text,
    embedding::text
FROM chunk_bge_m3
WHERE (%s)
ORDER BY updated_at DESC
LIMIT $%d
`, strings.Join(parts, " OR "), argIdx)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func scanChunks(rows *sql.Rows) ([]domain.Chunk, error) {
	out := make([]domain.Chunk, 0)
	for rows.Next() {
		var (
			chunk         domain.Chunk
			metadataText  sql.NullString
			embeddingText sql.NullString
		)
		if err := rows.Scan(
			&chunk.ID,
			&chunk.KBID,
			&chunk.DocID,
			&chunk.Content,
			&metadataText,
			&embeddingText,
		); err != nil {
			return nil, err
		}
		if metadataText.Valid {
			chunk.Metadata = metadataText.String
		}
		if embeddingText.Valid {
			vec, err := parseVectorText(embeddingText.String)
			if err != nil {
				// Keep retrieval resilient: skip malformed embedding row.
				continue
			}
			chunk.Embedding = vec
		}
		out = append(out, chunk)
	}
	return out, rows.Err()
}

func parseVectorText(s string) ([]float32, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil, nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if strings.TrimSpace(raw) == "" {
		return []float32{}, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, err
		}
		out = append(out, float32(v))
	}
	return out, nil
}
