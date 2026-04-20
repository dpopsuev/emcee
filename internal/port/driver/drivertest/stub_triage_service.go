package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.TriageService = (*StubTriageService)(nil)

type TriageCall struct {
	Ref      string
	MaxDepth int
}

type StubTriageService struct {
	Graph *domain.TriageGraph
	Err   error

	mu          sync.Mutex
	TriageCalls []TriageCall
}

func (s *StubTriageService) Triage(_ context.Context, ref string, maxDepth int) (*domain.TriageGraph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TriageCalls = append(s.TriageCalls, TriageCall{Ref: ref, MaxDepth: maxDepth})
	return s.Graph, s.Err
}
