package driven

import (
	"fmt"
	"sort"
	"sync"

	"github.com/DanyPops/emcee/internal/adapter/driven/cache"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// Factory creates an IssueRepository from a backend configuration entry.
// Return (nil, nil) if the backend is not applicable (e.g., missing credentials).
// Return (nil, err) if configuration is invalid.
type Factory func(name string, backend config.Backend) (driven.IssueRepository, error)

type entry struct {
	name     string
	priority int
	factory  Factory
}

var (
	mu       sync.Mutex
	registry []entry
)

// Register adds a backend factory to the global registry.
// Name must match the key used in config.yaml (e.g., "linear", "github").
// Higher priority factories are tried first.
func Register(name string, priority int, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, entry{name: name, priority: priority, factory: factory})
	sort.Slice(registry, func(i, j int) bool {
		return registry[i].priority > registry[j].priority
	})
}

// Available returns the names of all registered backends.
func Available() []string {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, len(registry))
	for i, e := range registry {
		names[i] = e.name
	}
	return names
}

// CreateFromConfig creates repositories for all configured backends.
// Returns repos and warnings (non-fatal errors for skipped backends).
func CreateFromConfig(cfg *config.Config) (repos []driven.IssueRepository, warnings []string) {
	mu.Lock()
	entries := make([]entry, len(registry))
	copy(entries, registry)
	mu.Unlock()

	for name, backend := range cfg.Backends {
		backendType := backend.ResolveType(name)
		var found bool
		for _, e := range entries {
			if e.name != backendType {
				continue
			}
			found = true
			repo, err := e.factory(name, backend)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
				break
			}
			if repo != nil {
				repos = append(repos, cache.New(repo))
			}
			break
		}
		if !found {
			warnings = append(warnings, fmt.Sprintf("unknown backend type %q for %q (available: %v)", backendType, name, availableNames(entries)))
		}
	}
	return repos, warnings
}

// CreateFromEnv creates repositories by checking environment variables.
// Each registered factory is called with an empty Backend; the factory
// checks env vars directly.
func CreateFromEnv() (repos []driven.IssueRepository, warnings []string) {
	mu.Lock()
	entries := make([]entry, len(registry))
	copy(entries, registry)
	mu.Unlock()

	for _, e := range entries {
		repo, err := e.factory(e.name, config.Backend{})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", e.name, err))
			continue
		}
		if repo != nil {
			repos = append(repos, cache.New(repo))
		}
	}
	return repos, warnings
}

// Reset clears the registry. Only for testing.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
}

func availableNames(entries []entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}
