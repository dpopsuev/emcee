// Package manifest manages per-backend manifests — YAML files that map
// semantic names to backend-specific values. Used for both field ID discovery
// (e.g. "sprint" → "customfield_10020") and status mapping
// (e.g. "ON_QA" → "in_review").
//
// File location: ~/.config/emcee/<kind>/<backend-name>.yaml
//
// Workflow:
//  1. Run the <kind>_discover MCP action once per backend to populate the manifest.
//  2. The manifest is loaded at startup and passed into the adapter.
//  3. Config entries override individual manifest entries.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultKind = "fields"

// Manifest holds the mapping from semantic field names to backend field IDs.
type Manifest struct {
	// Backend is the name this manifest belongs to (e.g. "jira").
	Backend string `yaml:"backend"`
	// DiscoveredAt is the timestamp of the last successful discovery run.
	DiscoveredAt time.Time `yaml:"discovered_at,omitempty"`
	// Mappings is the key-value store: semantic name → field ID.
	Mappings map[string]string `yaml:"mappings"`
}

// Get returns the field ID for a semantic name, or ("", false) if not found.
func (m *Manifest) Get(semantic string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m.Mappings[semantic]
	return v, ok
}

// Merge returns a new Manifest with overrides applied on top of m.
// Values in overrides take precedence; m is not mutated.
func (m *Manifest) Merge(overrides map[string]string) *Manifest {
	merged := &Manifest{
		Backend:      m.Backend,
		DiscoveredAt: m.DiscoveredAt,
		Mappings:     make(map[string]string, len(m.Mappings)+len(overrides)),
	}
	for k, v := range m.Mappings {
		merged.Mappings[k] = v
	}
	for k, v := range overrides {
		merged.Mappings[k] = v
	}
	return merged
}

// Load reads the manifest for the given backend from the config directory.
// kind selects the subdirectory (e.g. "fields", "statuses").
// Returns an empty manifest (not an error) if the file does not exist yet.
func Load(kind, backend, configDir string) (*Manifest, error) {
	path := DefaultPath(kind, backend, configDir)
	// #nosec G304 — path is constructed from config.Dir() + backend name, not user input
	data, err := os.ReadFile(path) //nolint:gosec
	if os.IsNotExist(err) {
		return &Manifest{Backend: backend, Mappings: map[string]string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read field manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse field manifest %s: %w", path, err)
	}
	if m.Mappings == nil {
		m.Mappings = map[string]string{}
	}
	return &m, nil
}

// Save writes the manifest to the config directory, creating the subdirectory
// if it does not exist. It overwrites any existing manifest for the same backend.
func Save(kind, backend, configDir string, m *Manifest) error {
	dir := filepath.Join(configDir, kind)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create fields dir %s: %w", dir, err)
	}
	m.Backend = backend
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal field manifest: %w", err)
	}
	header := []byte("# Emcee " + kind + " manifest — " + backend + "\n" +
		"# Run `" + kind + "_discover --backend " + backend + "` to regenerate.\n" +
		"# Entries in config.yaml override individual mappings.\n\n")
	path := DefaultPath(kind, backend, configDir)
	if err := os.WriteFile(path, append(header, data...), 0o600); err != nil {
		return fmt.Errorf("write field manifest %s: %w", path, err)
	}
	return nil
}

// DefaultPath returns the filesystem path for a backend's manifest file.
func DefaultPath(kind, backend, configDir string) string {
	return filepath.Join(configDir, kind, backend+".yaml")
}

// Discover builds a Manifest from pre-mapped key→value pairs.
// Callers are responsible for filtering and transforming raw backend data
// into the desired mappings before calling Discover.
func Discover(backend string, mappings map[string]string) *Manifest {
	m := make(map[string]string, len(mappings))
	for k, v := range mappings {
		m[k] = v
	}
	return &Manifest{
		Backend:      backend,
		DiscoveredAt: time.Now().UTC(),
		Mappings:     m,
	}
}
