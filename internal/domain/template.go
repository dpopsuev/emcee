package domain

import (
	"regexp"
	"strings"
	"unicode"
)

// Template represents a description template for a project/issue-type pair.
// Templates are discovered by sampling existing issues and extracting the
// repeating section structure from their descriptions.
type Template struct {
	Project   string   `json:"project"`
	IssueType string   `json:"issue_type"`
	Sections  []string `json:"sections"`
	Body      string   `json:"body"`
}

// jiraCodeBlockRe strips Jira-style {code} wrappers.
var jiraCodeBlockRe = regexp.MustCompile(`(?s)\{code(?::[^}]*)?\}(.*?)\{code\}`)

// ExtractTemplateSections takes multiple issue descriptions and returns
// the section headers that appear in ALL of them, preserving the order
// from the first description. Returns nil if no common sections are found.
func ExtractTemplateSections(descs []string) []string {
	if len(descs) == 0 {
		return nil
	}

	var allSections [][]string
	for _, desc := range descs {
		sections := extractSections(desc)
		if len(sections) == 0 {
			continue
		}
		allSections = append(allSections, sections)
	}
	if len(allSections) == 0 {
		return nil
	}

	// Start with first description's sections, keep only those in all others.
	common := allSections[0]
	for _, other := range allSections[1:] {
		set := make(map[string]bool, len(other))
		for _, s := range other {
			set[s] = true
		}
		var filtered []string
		for _, s := range common {
			if set[s] {
				filtered = append(filtered, s)
			}
		}
		common = filtered
	}
	if len(common) == 0 {
		return nil
	}
	return common
}

// BuildTemplateBody produces the empty template body from a list of sections.
func BuildTemplateBody(sections []string) string {
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func extractSections(desc string) []string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil
	}
	// Strip Jira {code} wrappers
	if m := jiraCodeBlockRe.FindStringSubmatch(desc); len(m) > 1 {
		desc = strings.TrimSpace(m[1])
	}

	var sections []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(desc, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// A section header: starts with uppercase, ends with ":"
		// and the colon-terminated prefix is a label.
		idx := strings.Index(line, ":")
		if idx < 2 {
			continue
		}
		label := line[:idx+1]
		// Must start with uppercase letter
		if label[0] < 'A' || label[0] > 'Z' {
			continue
		}
		clean := isCleanLabel(label[:len(label)-1])
		if !clean {
			continue
		}
		if !seen[label] {
			seen[label] = true
			sections = append(sections, label)
		}
	}
	if len(sections) < 2 {
		return nil
	}
	return sections
}

func isCleanLabel(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '/' || r == '(' || r == ')' || r == ',' || r == '-' {
			continue
		}
		return false
	}
	return true
}
