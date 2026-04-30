package docparse

import (
	"regexp"
	"strings"

	"github.com/dpopsuev/emcee/internal/domain"
)

// GoDeclaration represents a Go type or function declared in a code block.
type GoDeclaration struct {
	Kind      string `json:"kind"` // type, func, const, var
	Name      string `json:"name"`
	Line      int    `json:"line"`
	SectionID string `json:"section_id"`
}

var goPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^type\s+(\w+)\s+`),
	regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`),
	regexp.MustCompile(`^const\s+(\w+)\s+`),
	regexp.MustCompile(`^var\s+(\w+)\s+`),
}

var goKinds = []string{"type", "func", "const", "var"}

// ExtractGoDeclarations finds Go type/func/const/var declarations in code blocks.
func ExtractGoDeclarations(tree *domain.DocumentTree) []GoDeclaration {
	var decls []GoDeclaration
	for i := range tree.CodeBlocks {
		cb := tree.CodeBlocks[i]
		if cb.Language != "go" && cb.Language != "golang" {
			continue
		}
		for lineOffset, line := range strings.Split(cb.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			for j, pat := range goPatterns {
				if m := pat.FindStringSubmatch(trimmed); len(m) > 1 {
					decls = append(decls, GoDeclaration{
						Kind:      goKinds[j],
						Name:      m[1],
						Line:      cb.Line + lineOffset + 1,
						SectionID: cb.SectionID,
					})
				}
			}
		}
	}
	return decls
}
