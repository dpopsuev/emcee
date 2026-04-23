// Package triage provides the link extraction and graph storage implementations
// for the defect lifecycle triage engine.
package triage

import (
	"context"
	"regexp"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.LinkExtractor = (*RegexLinkExtractor)(nil)

// Pattern defines a regex pattern that extracts cross-references from text.
type Pattern struct {
	Name    string
	Regex   *regexp.Regexp
	Backend string // target backend type (e.g. "jira", "github")
	RefFmt  string // format string: use $0 for full match, $1 for first group, etc.
}

// DefaultPatterns returns the standard set of cross-reference patterns.
func DefaultPatterns() []Pattern {
	return []Pattern{
		{
			Name:    "jira_key",
			Regex:   regexp.MustCompile(`\b([A-Z][A-Z0-9]+-\d+)\b`),
			Backend: "jira",
			RefFmt:  "jira:$1",
		},
		{
			Name:    "github_pr",
			Regex:   regexp.MustCompile(`github\.com/([^/]+/[^/]+)/pull/(\d+)`),
			Backend: "github",
			RefFmt:  "github:$1#$2",
		},
		{
			Name:    "gitlab_mr",
			Regex:   regexp.MustCompile(`gitlab[^/]*/([^/]+(?:/[^/]+)*?)(?:/\-)?/merge_requests/(\d+)`),
			Backend: "gitlab",
			RefFmt:  "gitlab:$1!$2",
		},
		{
			Name:    "jenkins_build",
			Regex:   regexp.MustCompile(`jenkins[^/]*/job/([^/]+)/(\d+)`),
			Backend: "jenkins",
			RefFmt:  "jenkins:$1#$2",
		},
		{
			Name:    "rp_launch",
			Regex:   regexp.MustCompile(`launches/all/(\d+)`),
			Backend: "reportportal",
			RefFmt:  "reportportal:launch/$1",
		},
	}
}

// RegexLinkExtractor discovers cross-references in text using regex patterns.
type RegexLinkExtractor struct {
	patterns []Pattern
}

// NewRegexLinkExtractor creates an extractor with the given patterns.
// If patterns is nil, DefaultPatterns() is used.
func NewRegexLinkExtractor(patterns []Pattern) *RegexLinkExtractor {
	if patterns == nil {
		patterns = DefaultPatterns()
	}
	return &RegexLinkExtractor{patterns: patterns}
}

// Extract finds all cross-references in the given text.
func (e *RegexLinkExtractor) Extract(_ context.Context, text string) ([]domain.CrossRef, error) {
	seen := make(map[string]bool)
	var refs []domain.CrossRef

	for _, p := range e.patterns {
		matches := p.Regex.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			ref := []byte{}
			ref = p.Regex.ExpandString(ref, p.RefFmt, text, match)
			refStr := string(ref)

			if seen[refStr] {
				continue
			}
			seen[refStr] = true

			refs = append(refs, domain.CrossRef{
				Ref:        refStr,
				Source:     p.Name,
				Confidence: 0.9,
			})
		}
	}

	return refs, nil
}
