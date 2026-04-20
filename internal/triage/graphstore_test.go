package triage

import (
	"context"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
)

func TestInMemoryGraphStore_PutAndGet(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	_ = s.PutNode(ctx, domain.TriageNode{Ref: "jira:BUG-1", Title: "Bug One"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "github:org/repo#42", Title: "PR 42"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "jira:BUG-1", To: "github:org/repo#42", Type: "fixed_by"})

	graph, err := s.GetGraph(ctx, "jira:BUG-1", 3)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if graph.Seed != "jira:BUG-1" {
		t.Errorf("seed = %q, want %q", graph.Seed, "jira:BUG-1")
	}
	if len(graph.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(graph.Edges))
	}
}

func TestInMemoryGraphStore_BFSDepthLimit(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	// Chain: A → B → C → D
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "A"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "B"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "C"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "D"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "A", To: "B"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "B", To: "C"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "C", To: "D"})

	// Depth 1: should get A + B
	graph, _ := s.GetGraph(ctx, "A", 1)
	if len(graph.Nodes) != 2 {
		t.Errorf("depth=1: nodes = %d, want 2 (A, B)", len(graph.Nodes))
	}

	// Depth 2: should get A + B + C
	graph, _ = s.GetGraph(ctx, "A", 2)
	if len(graph.Nodes) != 3 {
		t.Errorf("depth=2: nodes = %d, want 3 (A, B, C)", len(graph.Nodes))
	}

	// Depth 3: should get all 4
	graph, _ = s.GetGraph(ctx, "A", 3)
	if len(graph.Nodes) != 4 {
		t.Errorf("depth=3: nodes = %d, want 4 (A, B, C, D)", len(graph.Nodes))
	}
}

func TestInMemoryGraphStore_NoCycles(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	// Cycle: A → B → A
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "A"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "B"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "A", To: "B"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "B", To: "A"})

	graph, _ := s.GetGraph(ctx, "A", 10)
	if len(graph.Nodes) != 2 {
		t.Errorf("cycle: nodes = %d, want 2 (no infinite loop)", len(graph.Nodes))
	}
}

func TestInMemoryGraphStore_UnknownSeed(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	graph, err := s.GetGraph(ctx, "nonexistent", 3)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if len(graph.Nodes) != 0 {
		t.Errorf("nodes = %d, want 0 for unknown seed", len(graph.Nodes))
	}
}

func TestInMemoryGraphStore_Bidirectional(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	_ = s.PutNode(ctx, domain.TriageNode{Ref: "A"})
	_ = s.PutNode(ctx, domain.TriageNode{Ref: "B"})
	_ = s.PutEdge(ctx, domain.TriageEdge{From: "A", To: "B"})

	// Traverse from B should still find A (edges are bidirectional in BFS)
	graph, _ := s.GetGraph(ctx, "B", 1)
	if len(graph.Nodes) != 2 {
		t.Errorf("bidirectional: nodes = %d, want 2", len(graph.Nodes))
	}
}
