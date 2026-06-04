package poller

import "time"

// Cursor is a durable per-poller bookmark for delta sync.
// Get returns the zero time if the poller has never run.
// Set persists the new position immediately and atomically.
type Cursor interface {
	Get(name string) time.Time
	Set(name string, t time.Time) error
}

// NopCursor is a no-op in-memory Cursor used in tests and non-serve commands.
// Positions are not persisted across calls.
type NopCursor struct {
	positions map[string]time.Time
}

func NewNopCursor() *NopCursor {
	return &NopCursor{positions: make(map[string]time.Time)}
}

func (c *NopCursor) Get(name string) time.Time {
	return c.positions[name]
}

func (c *NopCursor) Set(name string, t time.Time) error {
	c.positions[name] = t
	return nil
}
