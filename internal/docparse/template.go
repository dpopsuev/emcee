package docparse

import (
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// TemplateRule defines a required section in a document template.
type TemplateRule struct {
	Title    string `json:"title"`
	Required bool   `json:"required"`
}

// ValidationResult reports which required sections are present or missing.
type ValidationResult struct {
	Valid   bool     `json:"valid"`
	Present []string `json:"present"`
	Missing []string `json:"missing"`
}

// ValidateTemplate checks whether a document contains all required sections.
func ValidateTemplate(tree *domain.DocumentTree, rules []TemplateRule) ValidationResult {
	slugs := collectAllTitles(tree.Sections)
	result := ValidationResult{Valid: true}

	for _, rule := range rules {
		found := false
		for _, t := range slugs {
			if strings.EqualFold(t, rule.Title) {
				found = true
				break
			}
		}
		if found {
			result.Present = append(result.Present, rule.Title)
		} else if rule.Required {
			result.Missing = append(result.Missing, rule.Title)
			result.Valid = false
		}
	}
	return result
}

func collectAllTitles(sections []domain.Section) []string {
	var titles []string
	var walk func([]domain.Section)
	walk = func(ss []domain.Section) {
		for i := range ss {
			titles = append(titles, ss[i].Title)
			walk(ss[i].Children)
		}
	}
	walk(sections)
	return titles
}
