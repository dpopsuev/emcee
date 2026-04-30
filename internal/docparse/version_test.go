package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestVersionDiff(t *testing.T) {
	oldDoc := "# Doc\n\n## Intro\n\nText\n\n## Methods\n\nMore\n"
	newDoc := "# Doc\n\n## Intro\n\nText\n\n## Results\n\nNew section\n"

	oldTree := docparse.Parse([]byte(oldDoc))
	newTree := docparse.Parse([]byte(newDoc))

	diffs := docparse.VersionDiff(oldTree, newTree)

	var hasRemoved, hasAdded bool
	for _, d := range diffs {
		if d.Type == "removed" && d.Title == "Methods" {
			hasRemoved = true
		}
		if d.Type == "added" && d.Title == "Results" {
			hasAdded = true
		}
	}
	if !hasRemoved {
		t.Error("expected 'Methods' to be removed")
	}
	if !hasAdded {
		t.Error("expected 'Results' to be added")
	}
}

func TestVersionHeader(t *testing.T) {
	source := "# Doc\n\nContent\n"
	result := docparse.VersionHeader(source, "v3")
	if docparse.ExtractVersion(result) != "v3" {
		t.Errorf("version = %q, want v3", docparse.ExtractVersion(result))
	}
}

func TestExtractVersionEmpty(t *testing.T) {
	if v := docparse.ExtractVersion("# No version\n"); v != "" {
		t.Errorf("version = %q, want empty", v)
	}
}
