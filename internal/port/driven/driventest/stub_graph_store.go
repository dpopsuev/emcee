package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.GraphStore = (*StubGraphStore)(nil)

type StubGraphStore struct {
	Graph *domain.TriageGraph
	Err   error

	mu            sync.Mutex
	PutNodeCalls  []domain.TriageNode
	PutEdgeCalls  []domain.TriageEdge
	GetGraphCalls []GetGraphCall
}

type GetGraphCall struct {
	Seed     string
	MaxDepth int
}

func (s *StubGraphStore) PutNode(_ context.Context, node domain.TriageNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PutNodeCalls = append(s.PutNodeCalls, node)
	return s.Err
}

func (s *StubGraphStore) PutEdge(_ context.Context, edge domain.TriageEdge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PutEdgeCalls = append(s.PutEdgeCalls, edge)
	return s.Err
}

func (s *StubGraphStore) GetGraph(_ context.Context, seed string, maxDepth int) (*domain.TriageGraph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetGraphCalls = append(s.GetGraphCalls, GetGraphCall{Seed: seed, MaxDepth: maxDepth})
	return s.Graph, s.Err
}
