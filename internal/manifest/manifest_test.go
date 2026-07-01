package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSave_FieldsDirectory(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		Backend:      "jira",
		DiscoveredAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Mappings:     map[string]string{"sprint": "customfield_10020"},
	}
	if err := Save("fields", "jira", dir, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fields", "jira.yaml")); err != nil {
		t.Fatalf("expected fields/jira.yaml to exist: %v", err)
	}

	loaded, err := Load("fields", "jira", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok := loaded.Get("sprint"); !ok || v != "customfield_10020" {
		t.Errorf("got %q, %v; want customfield_10020, true", v, ok)
	}
}

func TestLoadSave_StatusesDirectory(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		Backend:      "jira",
		DiscoveredAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Mappings:     map[string]string{"ON_QA": "in_review", "MODIFIED": "in_review"},
	}
	if err := Save("statuses", "jira", dir, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "statuses", "jira.yaml")); err != nil {
		t.Fatalf("expected statuses/jira.yaml to exist: %v", err)
	}

	loaded, err := Load("statuses", "jira", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok := loaded.Get("ON_QA"); !ok || v != "in_review" {
		t.Errorf("got %q, %v; want in_review, true", v, ok)
	}
}

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	m, err := Load("statuses", "jira", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Mappings) != 0 {
		t.Errorf("expected empty mappings, got %v", m.Mappings)
	}
}

func TestDefaultPath_UsesKind(t *testing.T) {
	dir := t.TempDir()
	got := DefaultPath("statuses", "jira", dir)
	want := filepath.Join(dir, "statuses", "jira.yaml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMerge(t *testing.T) {
	m := &Manifest{
		Backend:  "jira",
		Mappings: map[string]string{"a": "1", "b": "2"},
	}
	merged := m.Merge(map[string]string{"b": "override", "c": "3"})
	if merged.Mappings["a"] != "1" {
		t.Error("expected a=1 preserved")
	}
	if merged.Mappings["b"] != "override" {
		t.Error("expected b overridden")
	}
	if merged.Mappings["c"] != "3" {
		t.Error("expected c=3 added")
	}
	if m.Mappings["b"] != "2" {
		t.Error("original mutated")
	}
}

func TestDiscover_AcceptsPreMappedEntries(t *testing.T) {
	mappings := map[string]string{
		"Sprint": "customfield_10020",
		"ON_QA":  "in_review",
	}
	m := Discover("jira", mappings)
	if v, ok := m.Get("Sprint"); !ok || v != "customfield_10020" {
		t.Errorf("Sprint: got %q, %v", v, ok)
	}
	if v, ok := m.Get("ON_QA"); !ok || v != "in_review" {
		t.Errorf("ON_QA: got %q, %v", v, ok)
	}
	if m.Backend != "jira" {
		t.Errorf("backend: got %q", m.Backend)
	}
	if m.DiscoveredAt.IsZero() {
		t.Error("DiscoveredAt should be set")
	}
}
