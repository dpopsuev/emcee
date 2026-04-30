package docparse

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

var errSectionNotFound = errors.New("section not found")

// MoveSection moves a section (by ID) to a new position after targetID.
func MoveSection(source string, tree *domain.DocumentTree, sectionID, afterID string) (string, error) {
	src := findSection(tree.Sections, sectionID)
	if src == nil {
		return "", fmt.Errorf("%w: %s", errSectionNotFound, sectionID)
	}
	tgt := findSection(tree.Sections, afterID)
	if tgt == nil {
		return "", fmt.Errorf("%w: %s", errSectionNotFound, afterID)
	}

	lines := strings.Split(source, "\n")
	srcLines := extractLines(lines, src.StartLine, src.EndLine)
	remaining := removeLines(lines, src.StartLine, src.EndLine)

	insertAt := tgt.EndLine
	if src.StartLine < tgt.StartLine {
		insertAt -= (src.EndLine - src.StartLine + 1)
	}

	result := insertAfterLine(remaining, insertAt, srcLines)
	return strings.Join(result, "\n"), nil
}

// MergeSection merges the content of fromID into toID, removing the from section.
func MergeSection(source string, tree *domain.DocumentTree, fromID, toID string) (string, error) {
	from := findSection(tree.Sections, fromID)
	if from == nil {
		return "", fmt.Errorf("%w: %s", errSectionNotFound, fromID)
	}
	to := findSection(tree.Sections, toID)
	if to == nil {
		return "", fmt.Errorf("%w: %s", errSectionNotFound, toID)
	}

	lines := strings.Split(source, "\n")
	fromContent := extractLines(lines, from.StartLine+1, from.EndLine) // skip heading
	remaining := removeLines(lines, from.StartLine, from.EndLine)

	insertAt := to.EndLine
	if from.StartLine < to.StartLine {
		insertAt -= (from.EndLine - from.StartLine + 1)
	}

	result := insertAfterLine(remaining, insertAt, fromContent)
	return strings.Join(result, "\n"), nil
}

func findSection(sections []domain.Section, id string) *domain.Section {
	for i := range sections {
		if sections[i].ID == id {
			return &sections[i]
		}
		if found := findSection(sections[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

func extractLines(lines []string, start, end int) []string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	return append([]string{}, lines[start-1:end]...)
}

func removeLines(lines []string, start, end int) []string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	result := make([]string, 0, len(lines)-(end-start+1))
	result = append(result, lines[:start-1]...)
	result = append(result, lines[end:]...)
	return result
}

func insertAfterLine(lines []string, after int, insert []string) []string {
	if after > len(lines) {
		after = len(lines)
	}
	result := make([]string, 0, len(lines)+len(insert))
	result = append(result, lines[:after]...)
	result = append(result, insert...)
	result = append(result, lines[after:]...)
	return result
}
