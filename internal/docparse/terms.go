package docparse

import (
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// TermUsage tracks where a term appears across sections.
type TermUsage struct {
	Term     string   `json:"term"`
	Sections []string `json:"sections"`
	Count    int      `json:"count"`
}

// FindTermUsage finds all sections containing the given term (case-insensitive).
func FindTermUsage(source string, tree *domain.DocumentTree, term string) TermUsage {
	lines := strings.Split(source, "\n")
	lower := strings.ToLower(term)
	usage := TermUsage{Term: term}
	seen := make(map[string]bool)

	for i, line := range lines {
		lineNum := i + 1
		if strings.Contains(strings.ToLower(line), lower) {
			usage.Count++
			secID := findSectionForLine(tree.Sections, lineNum)
			if secID != "" && !seen[secID] {
				seen[secID] = true
				usage.Sections = append(usage.Sections, secID)
			}
		}
	}
	return usage
}

// RenameTerm replaces all occurrences of oldTerm with newTerm in the source.
func RenameTerm(source, oldTerm, newTerm string) string {
	return strings.ReplaceAll(source, oldTerm, newTerm)
}

// FindInconsistentTerms detects terms that appear in multiple casings.
func FindInconsistentTerms(source string, terms []string) []TermInconsistency {
	var results []TermInconsistency
	for _, term := range terms {
		variants := make(map[string]int)
		for _, word := range strings.Fields(source) {
			clean := strings.Trim(word, ".,;:!?\"'()[]{}#@`*_")
			if strings.EqualFold(clean, term) {
				variants[clean]++
			}
		}
		if len(variants) > 1 {
			var vs []string
			for v := range variants {
				vs = append(vs, v)
			}
			results = append(results, TermInconsistency{
				Term:     term,
				Variants: vs,
			})
		}
	}
	return results
}

// TermInconsistency reports a term used with multiple casings.
type TermInconsistency struct {
	Term     string   `json:"term"`
	Variants []string `json:"variants"`
}
