package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

// SQLiteCursor implements poller.Cursor backed by the poller_cursors table
// in the same SQLite database as the ledger.
type SQLiteCursor struct {
	db *sql.DB
}

// NewCursor opens the cursor store from an existing *Ledger, sharing its DB.
func (l *Ledger) NewCursor() *SQLiteCursor {
	return &SQLiteCursor{db: l.db}
}

// Get returns the stored cursor position for name, or the zero time if not set.
func (c *SQLiteCursor) Get(name string) time.Time {
	var ts int64
	err := c.db.QueryRow(`SELECT cursor FROM poller_cursors WHERE name = ?`, name).Scan(&ts)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}

// Set persists the cursor position for name atomically via upsert.
func (c *SQLiteCursor) Set(name string, t time.Time) error {
	_, err := c.db.Exec(
		`INSERT INTO poller_cursors (name, cursor) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET cursor = excluded.cursor`,
		name, t.Unix(),
	)
	if err != nil {
		return fmt.Errorf("cursor set %q: %w", name, err)
	}
	return nil
}
