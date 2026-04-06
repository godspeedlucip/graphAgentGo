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
	q, args, empty := buildKeywordSearchAllKBQuery(keywords, limit)
	if empty {
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
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *PGVectorRepository) EnsurePerformanceIndexes(ctx context.Context) error {
	stmts := []string{
		// pgvector ANN index for similarity ORDER BY embedding <-> query
		`CREATE INDEX IF NOT EXISTS idx_chunk_bge_m3_embedding_ivfflat
ON chunk_bge_m3 USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		// supports kb scoped freshness fallback/order
		`CREATE INDEX IF NOT EXISTS idx_chunk_bge_m3_kb_updated_at
ON chunk_bge_m3 (kb_id, updated_at DESC)`,
		// supports global keyword empty-query ordering
		`CREATE INDEX IF NOT EXISTS idx_chunk_bge_m3_updated_at
ON chunk_bge_m3 (updated_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func buildKeywordSearchAllKBQuery(keywords []string, limit int) (query string, args []any, empty bool) {
	if len(keywords) == 0 {
		return "", nil, true
	}

	whereParts := make([]string, 0, len(keywords))
	scoreParts := make([]string, 0, len(keywords))
	args = make([]any, 0, len(keywords)+1)
	argIdx := 1

	for _, kw := range keywords {
		trimmed := strings.TrimSpace(kw)
		if trimmed == "" {
			continue
		}
		placeholder := fmt.Sprintf("$%d", argIdx)
		pattern := "%" + trimmed + "%"
		whereParts = append(whereParts, fmt.Sprintf("lower(content) LIKE lower(%s)", placeholder))
		// Align Java XML mapper:
		// CASE WHEN lower(content) LIKE ... THEN 1 ELSE 0 END (sum across keywords)
		scoreParts = append(scoreParts, fmt.Sprintf("CASE WHEN lower(content) LIKE lower(%s) THEN 1 ELSE 0 END", placeholder))
		args = append(args, pattern)
		argIdx++
	}

	if len(whereParts) == 0 {
		return "", nil, true
	}
	args = append(args, limit)

	query = fmt.Sprintf(`
SELECT
    id::text,
    kb_id::text,
    doc_id::text,
    content,
    metadata::text,
    embedding::text
FROM chunk_bge_m3
WHERE (%s)
ORDER BY (%s) DESC, updated_at DESC
LIMIT $%d
`, strings.Join(whereParts, " OR "), strings.Join(scoreParts, " + "), argIdx)
	return query, args, false
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
