package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestCheckDeadLinks(t *testing.T) {
	doc := "# Doc\n\n## Intro\n\n[good](#intro)\n\n[bad](#nonexistent)\n"
	tree := docparse.Parse([]byte(doc))
	docparse.CheckDeadLinks(tree)

	var good, bad bool
	for _, l := range tree.Links {
		if l.Destination == "#intro" && !l.Dead {
			good = true
		}
		if l.Destination == "#nonexistent" && l.Dead {
			bad = true
		}
	}
	if !good {
		t.Error("link to #intro should not be dead")
	}
	if !bad {
		t.Error("link to #nonexistent should be dead")
	}
}

func TestExtractLinkEdges(t *testing.T) {
	doc := "# Doc\n\n## Section A\n\nSee [section B](#section-b).\n\n## Section B\n\nContent.\n"
	tree := docparse.Parse([]byte(doc))
	edges := docparse.ExtractLinkEdges(tree)
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	if edges[0].Anchor != "section-b" {
		t.Errorf("anchor = %q, want section-b", edges[0].Anchor)
	}
}
