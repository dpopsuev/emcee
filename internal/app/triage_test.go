package app

import (
	"context"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
	"github.com/DanyPops/emcee/internal/triage"
)

func TestTriage_CrawlsAndExtractsLinks(t *testing.T) {
	// Setup: two issues that cross-reference each other via Jira keys in descriptions.
	repo := driventest.NewStubIssueRepository("test")
	repo.Issues = []domain.Issue{
		{
			Ref:         "test:BUG-1",
			Key:         "BUG-1",
			Title:       "Clock sync failure",
			Description: "Detected in jenkins build. See also RELATED-99 for context.",
			Status:      domain.StatusTodo,
		},
	}
	repo.Issue = &domain.Issue{
		Ref:         "test:BUG-1",
		Key:         "BUG-1",
		Title:       "Clock sync failure",
		Description: "Detected in jenkins build. See also RELATED-99 for context.",
		Status:      domain.StatusTodo,
	}

	svc := NewService(repo)
	svc.Apply(
		WithLinkExtractor(triage.NewRegexLinkExtractor(nil)),
		WithGraphStore(triage.NewInMemoryGraphStore()),
	)

	graph, err := svc.Triage(context.Background(), "test:BUG-1", 3)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}

	if graph.Seed != "test:BUG-1" {
		t.Errorf("seed = %q, want %q", graph.Seed, "test:BUG-1")
	}

	// Should have at least the seed node
	if len(graph.Nodes) < 1 {
		t.Fatalf("nodes = %d, want >= 1", len(graph.Nodes))
	}

	// Should have extracted RELATED-99 as a Jira cross-ref
	foundEdge := false
	for _, e := range graph.Edges {
		if e.To == "jira:RELATED-99" {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		t.Error("expected edge to jira:RELATED-99 from description extraction")
		for _, e := range graph.Edges {
			t.Logf("  edge: %s → %s (%s)", e.From, e.To, e.Type)
		}
	}
}

func TestTriage_NoGraphStore(t *testing.T) {
	repo := driventest.NewStubIssueRepository("test")
	svc := NewService(repo)
	// Don't inject graph store

	_, err := svc.Triage(context.Background(), "test:BUG-1", 3)
	if err == nil {
		t.Fatal("expected error when graph store not configured")
	}
}

func TestTriage_InvalidRef(t *testing.T) {
	repo := driventest.NewStubIssueRepository("test")
	svc := NewService(repo)
	svc.Apply(WithGraphStore(triage.NewInMemoryGraphStore()))

	graph, err := svc.Triage(context.Background(), "invalid", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid ref can't be parsed — crawl produces empty graph
	if len(graph.Nodes) != 0 {
		t.Errorf("nodes = %d, want 0 for unparseable ref", len(graph.Nodes))
	}
}
