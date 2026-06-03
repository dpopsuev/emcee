package fieldmanifest

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
	backend, configDir string,
	ttl time.Duration,
	discover func(ctx context.Context) ([]NamedField, error),
	apply func(map[string]string),
) *poller.Poller {
	isStale := func() bool {
		m, err := Load(backend, configDir)
		if err != nil || m.DiscoveredAt.IsZero() {
			return true
		}
		return time.Since(m.DiscoveredAt) >= ttl
	}

	refresh := func(ctx context.Context) error {
		fields, err := discover(ctx)
		if err != nil {
			return err
		}
		m := Discover(backend, fields)
		if err := Save(backend, configDir, m); err != nil {
			return err
		}
		apply(m.Mappings)
		return nil
	}

	return poller.New("fields:"+backend, isStale, refresh)
}
