package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestFindTermUsage(t *testing.T) {
	doc := "# Doc\n\n## Intro\n\nThe Clock is important.\n\n## Details\n\nThe clock runs fast.\n"
	tree := docparse.Parse([]byte(doc))
	usage := docparse.FindTermUsage(doc, tree, "clock")
	if usage.Count != 2 {
		t.Errorf("count = %d, want 2", usage.Count)
	}
	if len(usage.Sections) != 2 {
		t.Errorf("sections = %d, want 2", len(usage.Sections))
	}
}

func TestRenameTerm(t *testing.T) {
	source := "The PTP Clock is a PTP Clock."
	result := docparse.RenameTerm(source, "PTP Clock", "Boundary Clock")
	if result != "The Boundary Clock is a Boundary Clock." {
		t.Errorf("result = %q", result)
	}
}

func TestFindInconsistentTerms(t *testing.T) {
	source := "The Clock runs. The clock stops. The CLOCK resets."
	results := docparse.FindInconsistentTerms(source, []string{"clock"})
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if len(results[0].Variants) < 2 {
		t.Errorf("variants = %d, want >= 2", len(results[0].Variants))
	}
}
