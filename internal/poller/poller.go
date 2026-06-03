// Package poller provides a generic polling pattern: a startup staleness check
// and a background refresh loop. It is not tied to any domain concept — fields,
// launches, config files, or any other watchable resource can be expressed as a
// Poller by supplying isStale and refresh closures.
package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Watcher is the lifecycle interface for any resource that needs to be kept
// fresh. Check runs synchronously at startup; Run blocks until ctx is cancelled.
type Watcher interface {
	Check(ctx context.Context) error
	Run(ctx context.Context, interval time.Duration)
}

// Poller is the generic Watcher implementation.
//
// isStale reports whether the resource needs refreshing. For time-based TTLs it
// compares timestamps; for file watchers it compares mtimes; for immutable
// resources it always returns false.
//
// refresh performs the actual work: fetch from the backend, persist to disk,
// apply to the live in-memory state. A non-nil error is logged and the next
// tick retries.
type Poller struct {
	name    string
	isStale func() bool
	refresh func(ctx context.Context) error
}

// New constructs a Poller.
// name is used only for log attribution.
func New(name string, isStale func() bool, refresh func(ctx context.Context) error) *Poller {
	return &Poller{name: name, isStale: isStale, refresh: refresh}
}

// Check refreshes the resource if isStale returns true.
// Runs synchronously — the caller blocks until the refresh completes or fails.
func (p *Poller) Check(ctx context.Context) error {
	if !p.isStale() {
		return nil
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "poller: stale at startup, refreshing",
		slog.String("poller", p.name),
	)
	return p.refresh(ctx)
}

// Run starts the background refresh loop, firing every interval.
// isStale is rechecked on each tick so that cheap staleness guards (e.g. mtime
// checks for config files) avoid unnecessary refreshes.
// Blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.isStale() {
				continue
			}
			if err := p.refresh(ctx); err != nil {
				slog.LogAttrs(ctx, slog.LevelWarn, "poller: refresh failed",
					slog.String("poller", p.name),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

// --- Global registry ---
// Mirrors internal/infrastructure.Register: backends register Pollers in their
// init() functions; serveCmd iterates All() to run Check and launch goroutines.
// Registration is idempotent by name — re-registering replaces the existing
// entry, so a config reload that reconstructs backends does not duplicate loops.

var (
	mu       sync.Mutex
	registry = map[string]Watcher{}
)

// Register adds or replaces a named Watcher in the global registry.
func Register(name string, w Watcher) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = w
}

// All returns a snapshot of all registered Watchers in deterministic name order.
func All() []Watcher {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Watcher, 0, len(registry))
	for _, w := range registry {
		out = append(out, w)
	}
	return out
}

// Reset clears the registry. Only for testing.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]Watcher{}
}
