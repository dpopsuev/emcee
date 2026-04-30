package docparse

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// DuplicateBlock reports code blocks with identical content.
type DuplicateBlock struct {
	Hash      string   `json:"hash"`
	Language  string   `json:"language"`
	Locations []string `json:"locations"` // section IDs
	Lines     []int    `json:"lines"`
}

// FindDuplicateCodeBlocks detects code blocks with identical content.
func FindDuplicateCodeBlocks(tree *domain.DocumentTree) []DuplicateBlock {
	type entry struct {
		lang     string
		sections []string
		lines    []int
	}
	seen := make(map[string]*entry)
	for i := range tree.CodeBlocks {
		cb := tree.CodeBlocks[i]
		h := fmt.Sprintf("%x", sha256.Sum256([]byte(cb.Content)))[:12]
		if e, ok := seen[h]; ok {
			e.sections = append(e.sections, cb.SectionID)
			e.lines = append(e.lines, cb.Line)
		} else {
			seen[h] = &entry{
				lang:     cb.Language,
				sections: []string{cb.SectionID},
				lines:    []int{cb.Line},
			}
		}
	}

	var dups []DuplicateBlock
	for h, e := range seen {
		if len(e.sections) > 1 {
			dups = append(dups, DuplicateBlock{
				Hash:      h,
				Language:  e.lang,
				Locations: e.sections,
				Lines:     e.lines,
			})
		}
	}
	return dups
}

// SectionWeight reports the relative size of each section.
type SectionWeight struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Lines   int     `json:"lines"`
	Percent float64 `json:"percent"`
}

// AnalyzeBloat returns section weights sorted by size.
func AnalyzeBloat(tree *domain.DocumentTree) []SectionWeight {
	total := tree.LineCount
	if total == 0 {
		return nil
	}
	var weights []SectionWeight
	var walk func([]domain.Section)
	walk = func(ss []domain.Section) {
		for i := range ss {
			lines := ss[i].EndLine - ss[i].StartLine + 1
			weights = append(weights, SectionWeight{
				ID:      ss[i].ID,
				Title:   ss[i].Title,
				Lines:   lines,
				Percent: float64(lines) / float64(total) * 100,
			})
			walk(ss[i].Children)
		}
	}
	walk(tree.Sections)
	return weights
}

// ToneSweep finds lines containing any of the given negative terms.
type ToneMatch struct {
	Line    int    `json:"line"`
	Term    string `json:"term"`
	Context string `json:"context"`
}

// FindToneIssues scans for negative language patterns.
func FindToneIssues(source string, terms []string) []ToneMatch {
	lines := strings.Split(source, "\n")
	var matches []ToneMatch
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				ctx := line
				if len(ctx) > 120 {
					ctx = ctx[:117] + "..."
				}
				matches = append(matches, ToneMatch{
					Line:    i + 1,
					Term:    term,
					Context: ctx,
				})
			}
		}
	}
	return matches
}
