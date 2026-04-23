package driven

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
)

// LinkExtractor discovers cross-references in artifact text.
type LinkExtractor interface {
	// Extract finds cross-references (Jira keys, PR URLs, build URLs, etc.) in text.
	Extract(ctx context.Context, text string) ([]domain.CrossRef, error)
}

// GraphStore persists and queries the triage graph.
type GraphStore interface {
	// PutNode stores or updates a node in the graph.
	PutNode(ctx context.Context, node domain.TriageNode) error
	// PutEdge stores or updates an edge in the graph.
	PutEdge(ctx context.Context, edge domain.TriageEdge) error
	// GetGraph returns the subgraph reachable from a seed ref up to maxDepth hops.
	GetGraph(ctx context.Context, seed string, maxDepth int) (*domain.TriageGraph, error)
}
