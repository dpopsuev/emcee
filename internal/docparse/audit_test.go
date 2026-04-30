package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestFindDuplicateCodeBlocks(t *testing.T) {
	doc := "# Doc\n\n## A\n\n```go\nfmt.Println(\"hello\")\n```\n\n## B\n\n```go\nfmt.Println(\"hello\")\n```\n"
	tree := docparse.Parse([]byte(doc))
	dups := docparse.FindDuplicateCodeBlocks(tree)
	if len(dups) != 1 {
		t.Fatalf("dups = %d, want 1", len(dups))
	}
	if len(dups[0].Locations) != 2 {
		t.Errorf("locations = %d, want 2", len(dups[0].Locations))
	}
}

func TestAnalyzeBloat(t *testing.T) {
	doc := "# Doc\n\n## Short\n\nOne line.\n\n## Long\n\nLine 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	tree := docparse.Parse([]byte(doc))
	weights := docparse.AnalyzeBloat(tree)
	if len(weights) == 0 {
		t.Fatal("expected weights")
	}
	var totalPct float64
	for _, w := range weights {
		totalPct += w.Percent
	}
	if totalPct < 90 {
		t.Errorf("total percent = %.1f, expected ~100", totalPct)
	}
}

func TestFindToneIssues(t *testing.T) {
	source := "This is a good approach.\nThis is a terrible approach.\nNever do this.\n"
	matches := docparse.FindToneIssues(source, []string{"terrible", "never"})
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(matches))
	}
}
