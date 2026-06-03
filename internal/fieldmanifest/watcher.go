package fieldmanifest

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const DefaultTTL = 7 * 24 * time.Hour
const DefaultRefreshInterval = 24 * time.Hour

// Watcher keeps a field manifest evergreen: a startup TTL check and a
// background refresh loop. Both delegate to the same internal refresh.
type Watcher interface {
	// Check loads the on-disk manifest and re-discovers if it is older than
	// the configured TTL. Runs synchronously — callers block until done.
	Check(ctx context.Context) error
	// Run starts a background refresh loop that fires every interval.
	// Blocks until ctx is cancelled.
	Run(ctx context.Context, interval time.Duration)
}

// ManifestWatcher is the generic implementation. Backend-specific logic is
// injected via discover and apply so no import of backend packages is needed.
type ManifestWatcher struct {
	backend   string
	configDir string
	ttl       time.Duration
	// discover calls the backend API and returns the full field list.
	discover func(ctx context.Context) ([]NamedField, error)
	// apply hot-swaps the new mapping onto the live repository.
	apply func(map[string]string)
}

// NewWatcher constructs a ManifestWatcher.
// discover must call the backend's ListFields equivalent and return NamedField
// slices — the same input Discover() expects.
// apply is called with the new display_name→field_id map after every successful refresh.
func NewWatcher(
	backend, configDir string,
	ttl time.Duration,
	discover func(ctx context.Context) ([]NamedField, error),
	apply func(map[string]string),
) *ManifestWatcher {
	return &ManifestWatcher{
		backend:   backend,
		configDir: configDir,
		ttl:       ttl,
		discover:  discover,
		apply:     apply,
	}
}

// Check re-discovers if the on-disk manifest is older than the TTL.
func (w *ManifestWatcher) Check(ctx context.Context) error {
	m, err := Load(w.backend, w.configDir)
	if err != nil {
		return err
	}
	if !m.DiscoveredAt.IsZero() && time.Since(m.DiscoveredAt) < w.ttl {
		return nil
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "field manifest stale — refreshing",
		slog.String("backend", w.backend),
		slog.Duration("age", time.Since(m.DiscoveredAt)),
	)
	return w.refresh(ctx)
}

// Run starts the background refresh ticker. Blocks until ctx is cancelled.
func (w *ManifestWatcher) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.refresh(ctx); err != nil {
				slog.LogAttrs(ctx, slog.LevelWarn, "field manifest refresh failed",
					slog.String("backend", w.backend),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (w *ManifestWatcher) refresh(ctx context.Context) error {
	fields, err := w.discover(ctx)
	if err != nil {
		return err
	}
	m := Discover(w.backend, fields)
	if err := Save(w.backend, w.configDir, m); err != nil {
		return err
	}
	w.apply(m.Mappings)
	slog.LogAttrs(ctx, slog.LevelDebug, "field manifest refreshed",
		slog.String("backend", w.backend),
		slog.Int("fields", len(m.Mappings)),
	)
	return nil
}

// --- Global watcher registry ---
// Mirrors the infra.Register pattern: backends register watchers in init(),
// serveCmd iterates them to run Check and Run.

var (
	watcherMu       sync.Mutex
	watcherRegistry []Watcher
)

// RegisterWatcher adds a watcher to the global registry.
// Called from backend init() functions after the repository is constructed.
func RegisterWatcher(w Watcher) {
	watcherMu.Lock()
	defer watcherMu.Unlock()
	watcherRegistry = append(watcherRegistry, w)
}

// Watchers returns a snapshot of all registered watchers.
func Watchers() []Watcher {
	watcherMu.Lock()
	defer watcherMu.Unlock()
	out := make([]Watcher, len(watcherRegistry))
	copy(out, watcherRegistry)
	return out
}

// ResetWatchers clears the registry. Only for testing.
func ResetWatchers() {
	watcherMu.Lock()
	defer watcherMu.Unlock()
	watcherRegistry = nil
}
