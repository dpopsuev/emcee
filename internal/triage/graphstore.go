package triage

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.GraphStore = (*InMemoryGraphStore)(nil)

// InMemoryGraphStore is an in-memory graph with BFS traversal.
type InMemoryGraphStore struct {
	mu    sync.RWMutex
	nodes map[string]domain.TriageNode
	edges []domain.TriageEdge
}

// NewInMemoryGraphStore creates an empty graph store.
func NewInMemoryGraphStore() *InMemoryGraphStore {
	return &InMemoryGraphStore{
		nodes: make(map[string]domain.TriageNode),
	}
}

func (s *InMemoryGraphStore) PutNode(_ context.Context, node domain.TriageNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.Ref] = node
	return nil
}

func (s *InMemoryGraphStore) PutEdge(_ context.Context, edge domain.TriageEdge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges = append(s.edges, edge)
	return nil
}

// GetGraph returns the subgraph reachable from seed via BFS up to maxDepth hops.
func (s *InMemoryGraphStore) GetGraph(_ context.Context, seed string, maxDepth int) (*domain.TriageGraph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	visited := make(map[string]bool)
	queue := []struct {
		ref   string
		depth int
	}{{ref: seed, depth: 0}}
	visited[seed] = true

	var nodes []domain.TriageNode
	var edges []domain.TriageEdge

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if node, ok := s.nodes[curr.ref]; ok {
			nodes = append(nodes, node)
		}

		if curr.depth >= maxDepth {
			continue
		}

		for _, e := range s.edges {
			neighbor := ""
			if e.From == curr.ref {
				neighbor = e.To
			} else if e.To == curr.ref {
				neighbor = e.From
			}
			if neighbor == "" || visited[neighbor] {
				continue
			}
			visited[neighbor] = true
			edges = append(edges, e)
			queue = append(queue, struct {
				ref   string
				depth int
			}{ref: neighbor, depth: curr.depth + 1})
		}
	}

	return &domain.TriageGraph{
		Seed:  seed,
		Nodes: nodes,
		Edges: edges,
	}, nil
}
