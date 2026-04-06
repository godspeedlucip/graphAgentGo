package runlifecycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type OutboxRecorder interface {
	Record(ctx context.Context, topic string, payload any) error
}

type Repository struct {
	db     *sql.DB
	outbox OutboxRecorder
}

func NewRepository(db *sql.DB, outbox OutboxRecorder) (*Repository, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}
	return &Repository{db: db, outbox: outbox}, nil
}

func (r *Repository) Create(ctx context.Context, rec domain.RunRecord) error {
	metadata, err := toJSON(rec.Metadata)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO run_record (
    run_id, agent_id, session_id, status, input, output, error_code, error_msg, metadata, started_at, finished_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,CAST($9 AS jsonb),$10,$11)
`
	_, err = r.db.ExecContext(ctx, q,
		rec.RunID, rec.AgentID, rec.SessionID, string(rec.Status),
		rec.Input, rec.Output, rec.ErrorCode, rec.ErrorMsg, metadata, rec.StartedAt, rec.FinishedAt,
	)
	if err != nil {
		// TODO: normalize unique-constraint matching by driver/sqlstate.
		return err
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, runID string) (*domain.RunRecord, error) {
	const q = `
SELECT run_id, agent_id, session_id, status, input, output, error_code, error_msg, metadata::text, started_at, finished_at
FROM run_record WHERE run_id = $1
`
	var (
		rec          domain.RunRecord
		status       string
		metadataText sql.NullString
	)
	err := r.db.QueryRowContext(ctx, q, runID).Scan(
		&rec.RunID, &rec.AgentID, &rec.SessionID, &status, &rec.Input, &rec.Output,
		&rec.ErrorCode, &rec.ErrorMsg, &metadataText, &rec.StartedAt, &rec.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.Status = domain.Status(status)
	rec.Metadata, err = parseJSON(metadataText.String)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, runID string, status domain.Status, metadata map[string]any) error {
	metadataText, err := toJSON(metadata)
	if err != nil {
		return err
	}
	now := time.Now()
	const q = `
UPDATE run_record
SET status = $1, metadata = CAST($2 AS jsonb), finished_at = CASE WHEN $1 IN ('done','failed','canceled','timeout') THEN $3 ELSE finished_at END
WHERE run_id = $4
`
	res, err := r.db.ExecContext(ctx, q, string(status), metadataText, now, runID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrRunNotFound
	}
	// TODO: wire outbox transactional append (status update + outbox insert) in one DB tx.
	return nil
}

func (r *Repository) AppendOutput(ctx context.Context, runID string, delta string) error {
	if delta == "" {
		return nil
	}
	const q = `
UPDATE run_record
SET output = COALESCE(output,'') || $1
WHERE run_id = $2
`
	res, err := r.db.ExecContext(ctx, q, delta, runID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrRunNotFound
	}
	// TODO: wire outbox transactional append (output delta + outbox insert) in one DB tx.
	return nil
}

func toJSON(meta map[string]any) (string, error) {
	if len(meta) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseJSON(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
