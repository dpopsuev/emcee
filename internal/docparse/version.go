package docparse

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// VersionDiff compares two DocumentTrees and returns structural changes.
func VersionDiff(prev, curr *domain.DocumentTree) []DiffEntry {
	oldMap := flattenSections(prev.Sections)
	newMap := flattenSections(curr.Sections)

	var diffs []DiffEntry

	for slug, oldSec := range oldMap {
		if _, ok := newMap[slug]; !ok {
			diffs = append(diffs, DiffEntry{Type: "removed", Title: oldSec.Title, Level: oldSec.Level})
		}
	}
	for slug, newSec := range newMap {
		if _, ok := oldMap[slug]; !ok {
			diffs = append(diffs, DiffEntry{Type: "added", Title: newSec.Title, Level: newSec.Level})
		}
	}
	for slug, newSec := range newMap {
		if oldSec, ok := oldMap[slug]; ok {
			if oldSec.Level != newSec.Level {
				diffs = append(diffs, DiffEntry{
					Type:   "moved",
					Title:  newSec.Title,
					Level:  newSec.Level,
					Detail: fmt.Sprintf("level %d → %d", oldSec.Level, newSec.Level),
				})
			}
		}
	}

	return diffs
}

// DiffEntry describes a structural change between two document versions.
type DiffEntry struct {
	Type   string `json:"type"` // added, removed, moved
	Title  string `json:"title"`
	Level  int    `json:"level"`
	Detail string `json:"detail,omitempty"`
}

func flattenSections(sections []domain.Section) map[string]domain.Section {
	m := make(map[string]domain.Section)
	var walk func([]domain.Section)
	walk = func(ss []domain.Section) {
		for i := range ss {
			slug := slugify(ss[i].Title)
			m[slug] = ss[i]
			walk(ss[i].Children)
		}
	}
	walk(sections)
	return m
}

// VersionHeader inserts a version marker comment at the top of the document.
func VersionHeader(source, version string) string {
	header := fmt.Sprintf("<!-- version: %s -->\n", version)
	return header + source
}

// ExtractVersion reads the version marker from a document.
func ExtractVersion(source string) string {
	lines := strings.SplitN(source, "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	line := strings.TrimSpace(lines[0])
	if strings.HasPrefix(line, "<!-- version:") && strings.HasSuffix(line, "-->") {
		v := strings.TrimPrefix(line, "<!-- version:")
		v = strings.TrimSuffix(v, "-->")
		return strings.TrimSpace(v)
	}
	return ""
}
