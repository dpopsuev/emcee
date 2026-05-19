// Package fieldmanifest manages per-backend field manifests — YAML files that map
// semantic field names (e.g. "sprint") to backend-specific field IDs
// (e.g. "customfield_10020" on Jira). This decouples the application from
// hardcoded field IDs and lets each Jira instance (or other backend) have its own mapping.
//
// File location: ~/.config/emcee/fields/<backend-name>.yaml
//
// Workflow:
//  1. Run the fields_discover MCP action once per backend to populate the manifest.
//  2. The manifest is loaded at startup and passed into the adapter.
//  3. Entries in config.yaml backend.fields override individual manifest entries.
package fieldmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const manifestDir = "fields"

// KnownFields is the canonical list of semantic field names emcee understands,
// mapped to the display names used by Jira's field discovery API.
// Discovery matches these display names to resolve field IDs.
var KnownFields = map[string]string{
	"sprint":         "Sprint",
	"story_points":   "Story Points",
	"target_version": "Target Version",
}

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
// Returns an empty manifest (not an error) if the file does not exist yet.
func Load(backend, configDir string) (*Manifest, error) {
	path := DefaultPath(backend, configDir)
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

// Save writes the manifest to the config directory, creating the fields/ subdirectory
// if it does not exist. It overwrites any existing manifest for the same backend.
func Save(backend, configDir string, m *Manifest) error {
	dir := filepath.Join(configDir, manifestDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create fields dir %s: %w", dir, err)
	}
	m.Backend = backend
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal field manifest: %w", err)
	}
	header := []byte("# Emcee field manifest — " + backend + "\n" +
		"# Run `fields_discover --backend " + backend + "` to regenerate.\n" +
		"# Entries in config.yaml backend.fields override individual mappings.\n\n")
	path := DefaultPath(backend, configDir)
	if err := os.WriteFile(path, append(header, data...), 0o600); err != nil {
		return fmt.Errorf("write field manifest %s: %w", path, err)
	}
	return nil
}

// DefaultPath returns the filesystem path for a backend's manifest file.
func DefaultPath(backend, configDir string) string {
	return filepath.Join(configDir, manifestDir, backend+".yaml")
}

// Discover builds a Manifest by matching display names from allFields against
// KnownFields. allFields is the raw list returned by the backend's ListFields call.
// Only fields whose display name matches a KnownFields entry are included.
func Discover(backend string, allFields []NamedField) *Manifest {
	byName := make(map[string]string, len(allFields))
	for _, f := range allFields {
		byName[f.Name] = f.ID
	}
	mappings := make(map[string]string, len(KnownFields))
	for semantic, displayName := range KnownFields {
		if id, found := byName[displayName]; found {
			mappings[semantic] = id
		}
	}
	return &Manifest{
		Backend:      backend,
		DiscoveredAt: time.Now().UTC(),
		Mappings:     mappings,
	}
}

// NamedField is a minimal field descriptor used by Discover.
// It is intentionally backend-agnostic; adapters map their types to this.
type NamedField struct {
	ID   string
	Name string
}
