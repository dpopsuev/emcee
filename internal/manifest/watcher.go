package manifest

import (
	"context"
	"time"

	"github.com/dpopsuev/emcee/internal/poller"
)

const (
	DefaultTTL             = 7 * 24 * time.Hour
	DefaultRefreshInterval = 24 * time.Hour
)

// NewManifestPoller returns a *poller.Poller that keeps a field manifest
// evergreen for one backend.
//
// isStale: manifest.DiscoveredAt older than ttl.
// refresh: call discover, run Discover(), Save(), then apply the new mapping.
//
// discover must return the full field list from the backend API.
// apply hot-swaps the new display_name→field_id map onto the live repository.
func NewManifestPoller(
	kind, backend, configDir string,
	ttl time.Duration,
	discover func(ctx context.Context) (map[string]string, error),
	apply func(map[string]string),
) *poller.Poller {
	isStale := func() bool {
		m, err := Load(kind, backend, configDir)
		if err != nil || m.DiscoveredAt.IsZero() {
			return true
		}
		return time.Since(m.DiscoveredAt) >= ttl
	}

	refresh := func(ctx context.Context) error {
		mappings, err := discover(ctx)
		if err != nil {
			return err
		}
		m := Discover(backend, mappings)
		if err := Save(kind, backend, configDir, m); err != nil {
			return err
		}
		apply(m.Mappings)
		return nil
	}

	return poller.New(kind+":"+backend, isStale, refresh)
}
