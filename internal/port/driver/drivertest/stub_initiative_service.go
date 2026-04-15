//nolint:dupl // stub services share patterns by design
package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.InitiativeService = (*StubInitiativeService)(nil)

type InitListCall struct {
	Backend string
	Filter  domain.InitiativeListFilter
}

type InitCreateCall struct {
	Backend string
	Input   domain.InitiativeCreateInput
}

type StubInitiativeService struct {
	Initiatives []domain.Initiative
	Initiative  *domain.Initiative
	Err         error

	mu              sync.Mutex
	ListInitCalls   []InitListCall
	CreateInitCalls []InitCreateCall
}

func (s *StubInitiativeService) ListInitiatives(_ context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListInitCalls = append(s.ListInitCalls, InitListCall{Backend: backend, Filter: filter})
	return s.Initiatives, s.Err
}

func (s *StubInitiativeService) CreateInitiative(_ context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateInitCalls = append(s.CreateInitCalls, InitCreateCall{Backend: backend, Input: input})
	return s.Initiative, s.Err
}
