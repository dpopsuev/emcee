package docparse_test

import (
	"strings"
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestMoveSection(t *testing.T) {
	doc := "# Doc\n\n## A\n\nContent A\n\n## B\n\nContent B\n\n## C\n\nContent C\n"
	tree := docparse.Parse([]byte(doc))

	// Move A after C
	result, err := docparse.MoveSection(doc, tree, "s1.1", "s1.3")
	if err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	if !strings.Contains(result, "## B\n") {
		t.Error("B should still exist")
	}
	bIdx := strings.Index(result, "## B")
	aIdx := strings.Index(result, "## A")
	if aIdx < bIdx {
		t.Error("A should come after B after move")
	}
}

func TestMergeSection(t *testing.T) {
	doc := "# Doc\n\n## A\n\nContent A\n\n## B\n\nContent B\n"
	tree := docparse.Parse([]byte(doc))

	result, err := docparse.MergeSection(doc, tree, "s1.2", "s1.1")
	if err != nil {
		t.Fatalf("MergeSection: %v", err)
	}
	if strings.Contains(result, "## B") {
		t.Error("B heading should be removed after merge")
	}
	if !strings.Contains(result, "Content B") {
		t.Error("B content should be merged into A")
	}
}

func TestMoveSectionNotFound(t *testing.T) {
	doc := "# Doc\n\n## A\n\nContent\n"
	tree := docparse.Parse([]byte(doc))
	_, err := docparse.MoveSection(doc, tree, "nope", "s1.1")
	if err == nil {
		t.Error("expected error for missing section")
	}
}
