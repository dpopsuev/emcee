// Package sqlite provides a persistent Ledger backed by SQLite with FTS5.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.Ledger = (*Ledger)(nil)

// ErrNotFound is returned when an artifact ref is not in the ledger.
var ErrNotFound = errors.New("sqlite ledger: record not found")

// Ledger is a persistent artifact index backed by SQLite + FTS5.
type Ledger struct {
	db *sql.DB
}

// NewLedger opens (or creates) the SQLite ledger database at the given path.
// If path is empty, it defaults to $XDG_DATA_HOME/emcee/ledger.db.
func NewLedger(path string) (*Ledger, error) {
	if path == "" {
		path = defaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("sqlite ledger: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("sqlite ledger: open: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Ledger{db: db}, nil
}

// Close closes the underlying database connection.
func (l *Ledger) Close() error {
	return l.db.Close()
}

// Put upserts an artifact record.
func (l *Ledger) Put(ctx context.Context, r domain.ArtifactRecord) error {
	labels, _ := json.Marshal(r.Labels)
	components, _ := json.Marshal(r.Components)

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO artifacts (ref, backend, type, title, url, status, labels, components, text, seen_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ref) DO UPDATE SET
			backend=excluded.backend, type=excluded.type, title=excluded.title,
			url=excluded.url, status=excluded.status, labels=excluded.labels,
			components=excluded.components, text=excluded.text,
			seen_at=excluded.seen_at, updated_at=excluded.updated_at`,
		r.Ref, r.Backend, r.Type, r.Title, r.URL, r.Status,
		string(labels), string(components), r.Text,
		r.SeenAt.Unix(), r.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("sqlite ledger put: %w", err)
	}
	return nil
}

// Get returns a single artifact record by ref.
func (l *Ledger) Get(ctx context.Context, ref string) (*domain.ArtifactRecord, error) {
	row := l.db.QueryRowContext(ctx, `
		SELECT ref, backend, type, title, url, status, labels, components, text, seen_at, updated_at
		FROM artifacts WHERE ref = ?`, ref)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite ledger get: %w", err)
	}
	return r, nil
}

// List returns artifact records matching the filter.
func (l *Ledger) List(ctx context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error) {
	var clauses []string
	var args []any

	if filter.Backend != "" {
		clauses = append(clauses, "backend = ?")
		args = append(args, filter.Backend)
	}
	if filter.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Component != "" {
		clauses = append(clauses, "components LIKE ?")
		args = append(args, "%"+filter.Component+"%")
	}

	q := "SELECT ref, backend, type, title, url, status, labels, components, text, seen_at, updated_at FROM artifacts"
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY seen_at DESC"
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := l.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite ledger list: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

// Search performs full-text search across all artifact fields.
func (l *Ledger) Search(ctx context.Context, query string, limit int) ([]domain.ArtifactRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := l.db.QueryContext(ctx, `
		SELECT a.ref, a.backend, a.type, a.title, a.url, a.status,
		       a.labels, a.components, a.text, a.seen_at, a.updated_at
		FROM artifacts_fts f
		JOIN artifacts a ON a.ref = f.ref
		WHERE artifacts_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite ledger search: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

// Stats returns aggregate counts.
func (l *Ledger) Stats(ctx context.Context) (*domain.LedgerStats, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT backend, COUNT(*) FROM artifacts GROUP BY backend`)
	if err != nil {
		return nil, fmt.Errorf("sqlite ledger stats: %w", err)
	}
	defer rows.Close()

	stats := &domain.LedgerStats{ByBackend: make(map[string]int)}
	for rows.Next() {
		var backend string
		var count int
		if err := rows.Scan(&backend, &count); err != nil {
			return nil, fmt.Errorf("sqlite ledger stats scan: %w", err)
		}
		stats.ByBackend[backend] = count
		stats.Total += count
	}
	return stats, rows.Err()
}

// --- internal ---

func defaultPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "emcee", "ledger.db")
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS artifacts (
			ref        TEXT PRIMARY KEY,
			backend    TEXT NOT NULL,
			type       TEXT NOT NULL DEFAULT '',
			title      TEXT NOT NULL DEFAULT '',
			url        TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT '',
			labels     TEXT NOT NULL DEFAULT '[]',
			components TEXT NOT NULL DEFAULT '[]',
			text       TEXT NOT NULL DEFAULT '',
			seen_at    INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
			ref, backend, title, status, text,
			content=artifacts,
			content_rowid=rowid
		);
		CREATE TRIGGER IF NOT EXISTS artifacts_ai AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(rowid, ref, backend, title, status, text)
			VALUES (new.rowid, new.ref, new.backend, new.title, new.status, new.text);
		END;
		CREATE TRIGGER IF NOT EXISTS artifacts_ad AFTER DELETE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, ref, backend, title, status, text)
			VALUES ('delete', old.rowid, old.ref, old.backend, old.title, old.status, old.text);
		END;
		CREATE TRIGGER IF NOT EXISTS artifacts_au AFTER UPDATE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, ref, backend, title, status, text)
			VALUES ('delete', old.rowid, old.ref, old.backend, old.title, old.status, old.text);
			INSERT INTO artifacts_fts(rowid, ref, backend, title, status, text)
			VALUES (new.rowid, new.ref, new.backend, new.title, new.status, new.text);
		END;
	`)
	if err != nil {
		return fmt.Errorf("sqlite ledger migrate: %w", err)
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRecord(row scannable) (*domain.ArtifactRecord, error) {
	var r domain.ArtifactRecord
	var labelsJSON, componentsJSON string
	var seenAt, updatedAt int64

	err := row.Scan(&r.Ref, &r.Backend, &r.Type, &r.Title, &r.URL, &r.Status,
		&labelsJSON, &componentsJSON, &r.Text, &seenAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(labelsJSON), &r.Labels)
	_ = json.Unmarshal([]byte(componentsJSON), &r.Components)
	r.SeenAt = time.Unix(seenAt, 0)
	r.UpdatedAt = time.Unix(updatedAt, 0)
	return &r, nil
}

func scanRecords(rows *sql.Rows) ([]domain.ArtifactRecord, error) {
	var results []domain.ArtifactRecord
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite ledger scan: %w", err)
		}
		results = append(results, *r)
	}
	return results, rows.Err()
}
