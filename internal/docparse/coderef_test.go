package docparse_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/docparse"
)

func TestExtractGoDeclarations(t *testing.T) {
	doc := "# Spec\n\n## Types\n\n```go\ntype Clock struct {\n    Name string\n}\n\nfunc NewClock() *Clock {\n    return &Clock{}\n}\n\nconst MaxRetries = 3\n```\n"
	tree := docparse.Parse([]byte(doc))
	decls := docparse.ExtractGoDeclarations(tree)

	kinds := make(map[string]string)
	for _, d := range decls {
		kinds[d.Name] = d.Kind
	}

	if kinds["Clock"] != "type" {
		t.Errorf("Clock kind = %q, want type", kinds["Clock"])
	}
	if kinds["NewClock"] != "func" {
		t.Errorf("NewClock kind = %q, want func", kinds["NewClock"])
	}
	if kinds["MaxRetries"] != "const" {
		t.Errorf("MaxRetries kind = %q, want const", kinds["MaxRetries"])
	}
}

func TestExtractGoDeclarationsIgnoresNonGo(t *testing.T) {
	doc := "# Doc\n\n```python\ndef hello():\n    pass\n```\n"
	tree := docparse.Parse([]byte(doc))
	decls := docparse.ExtractGoDeclarations(tree)
	if len(decls) != 0 {
		t.Errorf("expected 0 Go decls from Python block, got %d", len(decls))
	}
}
