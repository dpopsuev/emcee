package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

const testDoc = `# My Document

Introduction paragraph.

## Section One

Some content here with a [link](https://example.com).

### Subsection 1.1

Details about subsection.

## Section Two

` + "```go" + `
type Clock struct {
    Name string
}
` + "```" + `

More text after code.

## Section Three

Final section with [internal](#section-one) link.
`

func TestParseTitle(t *testing.T) {
	tree := docparse.Parse([]byte(testDoc))
	if tree.Title != "My Document" {
		t.Errorf("Title = %q, want %q", tree.Title, "My Document")
	}
}

func TestParseSections(t *testing.T) {
	tree := docparse.Parse([]byte(testDoc))
	if len(tree.Sections) != 1 {
		t.Fatalf("root sections = %d, want 1 (h1 is root)", len(tree.Sections))
	}
	root := tree.Sections[0]
	if len(root.Children) != 3 {
		t.Fatalf("h2 children = %d, want 3", len(root.Children))
	}
	if root.Children[0].Title != "Section One" {
		t.Errorf("first child = %q, want %q", root.Children[0].Title, "Section One")
	}
	if len(root.Children[0].Children) != 1 {
		t.Fatalf("h3 children = %d, want 1", len(root.Children[0].Children))
	}
	if root.Children[0].Children[0].Title != "Subsection 1.1" {
		t.Errorf("subsection = %q", root.Children[0].Children[0].Title)
	}
}

func TestParseSectionIDs(t *testing.T) {
	tree := docparse.Parse([]byte(testDoc))
	root := tree.Sections[0]
	if root.ID != "s1" {
		t.Errorf("root ID = %q, want s1", root.ID)
	}
	if root.Children[0].ID != "s1.1" {
		t.Errorf("first child ID = %q, want s1.1", root.Children[0].ID)
	}
	if root.Children[0].Children[0].ID != "s1.1.1" {
		t.Errorf("subsection ID = %q, want s1.1.1", root.Children[0].Children[0].ID)
	}
}

func TestParseLinks(t *testing.T) {
	tree := docparse.Parse([]byte(testDoc))
	if len(tree.Links) != 2 {
		t.Fatalf("links = %d, want 2", len(tree.Links))
	}
	if tree.Links[0].Destination != "https://example.com" {
		t.Errorf("link[0] dest = %q", tree.Links[0].Destination)
	}
	if tree.Links[1].Destination != "#section-one" {
		t.Errorf("link[1] dest = %q", tree.Links[1].Destination)
	}
}

func TestParseCodeBlocks(t *testing.T) {
	tree := docparse.Parse([]byte(testDoc))
	if len(tree.CodeBlocks) != 1 {
		t.Fatalf("code blocks = %d, want 1", len(tree.CodeBlocks))
	}
	if tree.CodeBlocks[0].Language != "go" {
		t.Errorf("language = %q, want go", tree.CodeBlocks[0].Language)
	}
	if tree.CodeBlocks[0].SectionID == "" {
		t.Error("code block should be assigned to a section")
	}
}

func TestParseEmpty(t *testing.T) {
	tree := docparse.Parse([]byte(""))
	if tree.Title != "" {
		t.Errorf("empty doc title = %q", tree.Title)
	}
	if len(tree.Sections) != 0 {
		t.Errorf("empty doc sections = %d", len(tree.Sections))
	}
}

func TestParseNoHeadings(t *testing.T) {
	tree := docparse.Parse([]byte("Just a paragraph.\n\nAnother paragraph."))
	if len(tree.Sections) != 0 {
		t.Errorf("no-heading doc sections = %d", len(tree.Sections))
	}
}
