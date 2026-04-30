package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestValidateTemplatePass(t *testing.T) {
	doc := "# ADR\n\n## Context\n\nWhy.\n\n## Decision\n\nWhat.\n\n## Consequences\n\nImpact.\n"
	tree := docparse.Parse([]byte(doc))
	rules := []docparse.TemplateRule{
		{Title: "Context", Required: true},
		{Title: "Decision", Required: true},
		{Title: "Consequences", Required: true},
	}
	result := docparse.ValidateTemplate(tree, rules)
	if !result.Valid {
		t.Errorf("expected valid, missing: %v", result.Missing)
	}
	if len(result.Present) != 3 {
		t.Errorf("present = %d, want 3", len(result.Present))
	}
}

func TestValidateTemplateFail(t *testing.T) {
	doc := "# ADR\n\n## Context\n\nWhy.\n\n## Decision\n\nWhat.\n"
	tree := docparse.Parse([]byte(doc))
	rules := []docparse.TemplateRule{
		{Title: "Context", Required: true},
		{Title: "Decision", Required: true},
		{Title: "Consequences", Required: true},
	}
	result := docparse.ValidateTemplate(tree, rules)
	if result.Valid {
		t.Error("expected invalid — Consequences missing")
	}
	if len(result.Missing) != 1 || result.Missing[0] != "Consequences" {
		t.Errorf("missing = %v, want [Consequences]", result.Missing)
	}
}
