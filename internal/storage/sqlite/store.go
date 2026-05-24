package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Completion struct {
	ID              string
	RequestID       string
	FilePath        string
	LanguageID      string
	CompletionText  string
	TokensGenerated int
	FirstTokenMs    int
	Accepted        *bool
	AdapterVersion  string
	CreatedAt       time.Time
}

type FeedbackEvent struct {
	ID           string
	CompletionID string
	Action       string
	CreatedAt    time.Time
}

func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS completions (
			id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			request_id      TEXT NOT NULL UNIQUE,
			file_path       TEXT,
			language_id     TEXT,
			completion_text TEXT,
			tokens_generated INTEGER DEFAULT 0,
			first_token_ms  INTEGER DEFAULT 0,
			accepted        INTEGER,
			adapter_version TEXT DEFAULT 'base',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS feedback_events (
			id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			completion_id TEXT REFERENCES completions(id),
			action        TEXT CHECK(action IN ('accept','reject')),
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_completions_request_id ON completions(request_id);
		CREATE INDEX IF NOT EXISTS idx_feedback_completion_id ON feedback_events(completion_id);
	`)
	return err
}

func (s *Store) SaveCompletion(ctx context.Context, c Completion) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO completions (request_id, file_path, language_id, completion_text, tokens_generated, first_token_ms, adapter_version)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(request_id) DO UPDATE SET
			completion_text  = excluded.completion_text,
			tokens_generated = excluded.tokens_generated,
			first_token_ms   = excluded.first_token_ms
	`, c.RequestID, c.FilePath, c.LanguageID, c.CompletionText, c.TokensGenerated, c.FirstTokenMs, c.AdapterVersion)
	return err
}

func (s *Store) RecordFeedback(ctx context.Context, requestID, action string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var completionID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM completions WHERE request_id = ?", requestID).Scan(&completionID)
	if err != nil {
		return fmt.Errorf("completion not found for request_id %s: %w", requestID, err)
	}

	accepted := 0
	if action == "accept" {
		accepted = 1
	}

	_, err = tx.ExecContext(ctx, "UPDATE completions SET accepted = ? WHERE id = ?", accepted, completionID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO feedback_events (completion_id, action) VALUES (?, ?)
	`, completionID, action)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) GetStats(ctx context.Context) (total, accepted, rejected int, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM completions").Scan(&total)
	if err != nil {
		return
	}
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM completions WHERE accepted = 1").Scan(&accepted)
	if err != nil {
		return
	}
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM completions WHERE accepted = 0").Scan(&rejected)
	return
}

func (s *Store) Close() error {
	return s.db.Close()
}
