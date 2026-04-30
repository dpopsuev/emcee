// Package docparse extracts structural information from markdown documents
// using goldmark's AST. It produces a DocumentTree with sections, links,
// and code blocks without modifying the source text.
package docparse

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/dpopsuev/emcee/internal/domain"
)

// Parse parses markdown content and returns a DocumentTree.
func Parse(source []byte) *domain.DocumentTree {
	reader := text.NewReader(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	lines := bytes.Count(source, []byte("\n")) + 1
	tree := &domain.DocumentTree{LineCount: lines}

	var sections []flatSection
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			title := extractText(node, source)
			line := lineNumber(node, source)
			sections = append(sections, flatSection{
				title: title,
				level: node.Level,
				line:  line,
			})
			if node.Level == 1 && tree.Title == "" {
				tree.Title = title
			}
		case *ast.Link:
			tree.Links = append(tree.Links, domain.DocLink{
				Text:        extractText(node, source),
				Destination: string(node.Destination),
				Line:        lineNumber(node, source),
			})
		case *ast.AutoLink:
			tree.Links = append(tree.Links, domain.DocLink{
				Text:        string(node.URL(source)),
				Destination: string(node.URL(source)),
				Line:        lineNumber(node, source),
			})
		case *ast.FencedCodeBlock:
			cbLine := lineNumber(node, source)
			if cbLine > 0 {
				cbLine-- // fence line is one line before content
			}
			tree.CodeBlocks = append(tree.CodeBlocks, domain.CodeBlock{
				Language: string(node.Language(source)),
				Content:  extractCodeContent(node, source),
				Line:     cbLine,
			})
		}
		return ast.WalkContinue, nil
	})

	tree.Sections = buildTree(sections, lines)
	assignSectionIDs(tree.Sections, "")
	assignLinksToSections(tree)
	assignCodeBlocksToSections(tree)

	return tree
}

type flatSection struct {
	title string
	level int
	line  int
}

func buildTree(flat []flatSection, totalLines int) []domain.Section {
	if len(flat) == 0 {
		return nil
	}

	result := make([]domain.Section, 0, len(flat))
	for i, s := range flat {
		endLine := totalLines
		if i+1 < len(flat) {
			endLine = flat[i+1].line - 1
		}
		result = append(result, domain.Section{
			Title:     s.title,
			Level:     s.level,
			StartLine: s.line,
			EndLine:   endLine,
		})
	}

	return nestSections(result)
}

func nestSections(flat []domain.Section) []domain.Section {
	if len(flat) == 0 {
		return nil
	}

	var roots []domain.Section
	var stack []*domain.Section

	for i := range flat {
		s := flat[i]
		for len(stack) > 0 && stack[len(stack)-1].Level >= s.Level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, s)
			stack = []*domain.Section{&roots[len(roots)-1]}
		} else {
			parent := stack[len(stack)-1]
			if s.EndLine > parent.EndLine {
				parent.EndLine = s.EndLine
			}
			parent.Children = append(parent.Children, s)
			stack = append(stack, &parent.Children[len(parent.Children)-1])
		}
	}
	return roots
}

func assignSectionIDs(sections []domain.Section, prefix string) {
	for i := range sections {
		if prefix == "" {
			sections[i].ID = fmt.Sprintf("s%d", i+1)
		} else {
			sections[i].ID = fmt.Sprintf("%s.%d", prefix, i+1)
		}
		assignSectionIDs(sections[i].Children, sections[i].ID)
	}
}

func assignLinksToSections(tree *domain.DocumentTree) {
	for i := range tree.Links {
		tree.Links[i].SectionID = findSectionForLine(tree.Sections, tree.Links[i].Line)
	}
}

func assignCodeBlocksToSections(tree *domain.DocumentTree) {
	for i := range tree.CodeBlocks {
		tree.CodeBlocks[i].SectionID = findSectionForLine(tree.Sections, tree.CodeBlocks[i].Line)
	}
}

func findSectionForLine(sections []domain.Section, line int) string {
	for i := range sections {
		if line >= sections[i].StartLine && line <= sections[i].EndLine {
			if child := findSectionForLine(sections[i].Children, line); child != "" {
				return child
			}
			return sections[i].ID
		}
	}
	return ""
}

func extractText(n ast.Node, source []byte) string {
	var b strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return b.String()
}

func extractCodeContent(n *ast.FencedCodeBlock, source []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

func lineNumber(n ast.Node, source []byte) int {
	if n.Type() == ast.TypeBlock {
		lines := n.Lines()
		if lines.Len() > 0 {
			seg := lines.At(0)
			return bytes.Count(source[:seg.Start], []byte("\n")) + 1
		}
	}
	if fc := n.FirstChild(); fc != nil {
		if t, ok := fc.(*ast.Text); ok {
			return bytes.Count(source[:t.Segment.Start], []byte("\n")) + 1
		}
	}
	return 0
}
