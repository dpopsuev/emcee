package docparse

import (
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// CheckDeadLinks marks internal anchor links as dead if they don't match any section heading.
func CheckDeadLinks(tree *domain.DocumentTree) {
	slugs := collectSlugs(tree.Sections)
	for i := range tree.Links {
		dest := tree.Links[i].Destination
		if !strings.HasPrefix(dest, "#") {
			continue
		}
		anchor := strings.TrimPrefix(dest, "#")
		if _, ok := slugs[anchor]; !ok {
			tree.Links[i].Dead = true
		}
	}
}

// ExtractLinkEdges returns edges between sections based on internal links.
func ExtractLinkEdges(tree *domain.DocumentTree) []LinkEdge {
	slugToSection := collectSlugs(tree.Sections)
	var edges []LinkEdge
	for i := range tree.Links {
		link := tree.Links[i]
		if !strings.HasPrefix(link.Destination, "#") || link.SectionID == "" {
			continue
		}
		anchor := strings.TrimPrefix(link.Destination, "#")
		if targetID, ok := slugToSection[anchor]; ok && targetID != link.SectionID {
			edges = append(edges, LinkEdge{
				From:   link.SectionID,
				To:     targetID,
				Anchor: anchor,
				Line:   link.Line,
			})
		}
	}
	return edges
}

// LinkEdge represents a directed edge between two sections via an internal link.
type LinkEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
}

func collectSlugs(sections []domain.Section) map[string]string {
	slugs := make(map[string]string)
	var walk func([]domain.Section)
	walk = func(ss []domain.Section) {
		for i := range ss {
			slug := slugify(ss[i].Title)
			slugs[slug] = ss[i].ID
			walk(ss[i].Children)
		}
	}
	walk(sections)
	return slugs
}

func slugify(title string) string {
	title = strings.ToLower(title)
	var b strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteRune('-')
		}
	}
	return b.String()
}
