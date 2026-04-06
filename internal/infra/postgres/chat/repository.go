package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	domain "go-sse-skeleton/internal/domain/chat"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Create(ctx context.Context, msg *domain.Message) (string, error) {
	if msg == nil {
		return "", domain.ErrInvalidInput
	}
	metadata, err := toMetadataValue(msg.Metadata)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	msg.UpdatedAt = now

	const q = `
INSERT INTO chat_message (
    session_id,
    role,
    content,
    metadata,
    created_at,
    updated_at
) VALUES (
    CAST($1 AS uuid),
    $2,
    $3,
    CAST($4 AS jsonb),
    $5,
    $6
) RETURNING id::text
`

	var id string
	if err = r.db.QueryRowContext(
		ctx,
		q,
		msg.SessionID,
		string(msg.Role),
		msg.Content,
		metadata,
		msg.CreatedAt,
		msg.UpdatedAt,
	).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Message, error) {
	if id == "" {
		return nil, domain.ErrInvalidInput
	}
	const q = `
SELECT
    id::text,
    session_id::text,
    role,
    content,
    metadata::text,
    created_at,
    updated_at
FROM chat_message
WHERE id = CAST($1 AS uuid)
`

	var (
		msg          domain.Message
		metadataText sql.NullString
	)
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&msg.ID,
		&msg.SessionID,
		&msg.Role,
		&msg.Content,
		&metadataText,
		&msg.CreatedAt,
		&msg.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	msg.Metadata, err = parseMetadata(metadataText.String)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *Repository) ListBySession(ctx context.Context, sessionID string) ([]*domain.Message, error) {
	if sessionID == "" {
		return nil, domain.ErrInvalidInput
	}
	const q = `
SELECT
    id::text,
    session_id::text,
    role,
    content,
    metadata::text,
    created_at,
    updated_at
FROM chat_message
WHERE session_id = CAST($1 AS uuid)
ORDER BY created_at ASC
`

	rows, err := r.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *Repository) ListRecentBySession(ctx context.Context, sessionID string, limit int) ([]*domain.Message, error) {
	if sessionID == "" || limit <= 0 {
		return nil, domain.ErrInvalidInput
	}
	// Keep Java mapper semantics:
	// 1) inner query picks latest N in DESC
	// 2) outer query re-orders ASC for chronological replay
	const q = `
SELECT
    id::text,
    session_id::text,
    role,
    content,
    metadata::text,
    created_at,
    updated_at
FROM (
    SELECT
        id,
        session_id,
        role,
        content,
        metadata,
        created_at,
        updated_at
    FROM chat_message
    WHERE session_id = CAST($1 AS uuid)
    ORDER BY created_at DESC
    LIMIT $2
) t
ORDER BY created_at ASC
`

	rows, err := r.db.QueryContext(ctx, q, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *Repository) Update(ctx context.Context, msg *domain.Message) error {
	if msg == nil || msg.ID == "" {
		return domain.ErrInvalidInput
	}
	metadata, err := toMetadataValue(msg.Metadata)
	if err != nil {
		return err
	}
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = time.Now()
	}

	const q = `
UPDATE chat_message
SET
    session_id = CAST($1 AS uuid),
    role = $2,
    content = $3,
    metadata = CAST($4 AS jsonb),
    updated_at = $5
WHERE id = CAST($6 AS uuid)
`

	result, err := r.db.ExecContext(
		ctx,
		q,
		msg.SessionID,
		string(msg.Role),
		msg.Content,
		metadata,
		msg.UpdatedAt,
		msg.ID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) AppendContent(ctx context.Context, id string, delta string) error {
	if id == "" || delta == "" {
		return domain.ErrInvalidInput
	}

	// Atomic append in SQL to avoid read-modify-write lost update under concurrent stream deltas.
	// Adapter point: this query can be moved to sqlc/pgx generated layer without changing service contracts.
	const q = `
UPDATE chat_message
SET
    content = COALESCE(content, '') || $1,
    updated_at = $2
WHERE id = CAST($3 AS uuid)
`

	result, err := r.db.ExecContext(ctx, q, delta, time.Now(), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return domain.ErrInvalidInput
	}
	const q = `DELETE FROM chat_message WHERE id = CAST($1 AS uuid)`
	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanMessages(rows *sql.Rows) ([]*domain.Message, error) {
	out := make([]*domain.Message, 0)
	for rows.Next() {
		var (
			msg          domain.Message
			metadataText sql.NullString
		)
		if err := rows.Scan(
			&msg.ID,
			&msg.SessionID,
			&msg.Role,
			&msg.Content,
			&metadataText,
			&msg.CreatedAt,
			&msg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		metadata, err := parseMetadata(metadataText.String)
		if err != nil {
			return nil, err
		}
		msg.Metadata = metadata
		out = append(out, &msg)
	}
	return out, rows.Err()
}

func toMetadataValue(metadata *domain.Metadata) (any, error) {
	if metadata == nil {
		return nil, nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func parseMetadata(raw string) (*domain.Metadata, error) {
	if raw == "" {
		return nil, nil
	}
	var metadata domain.Metadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}
