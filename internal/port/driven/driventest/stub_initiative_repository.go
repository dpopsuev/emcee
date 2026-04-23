//nolint:dupl // stub repositories share patterns by design
package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.InitiativeRepository = (*StubInitiativeRepository)(nil)

type ListInitiativesCall struct {
	Filter domain.InitiativeListFilter
}

type CreateInitiativeCall struct {
	Input domain.InitiativeCreateInput
}

type StubInitiativeRepository struct {
	NameVal     string
	Initiatives []domain.Initiative
	Initiative  *domain.Initiative
	Err         error

	ListInitErr   error
	CreateInitErr error

	mu              sync.Mutex
	ListInitCalls   []ListInitiativesCall
	CreateInitCalls []CreateInitiativeCall
}

func (s *StubInitiativeRepository) Name() string { return s.NameVal }

func (s *StubInitiativeRepository) ListInitiatives(_ context.Context, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListInitCalls = append(s.ListInitCalls, ListInitiativesCall{Filter: filter})
	if s.ListInitErr != nil {
		return nil, s.ListInitErr
	}
	return s.Initiatives, s.Err
}

func (s *StubInitiativeRepository) CreateInitiative(_ context.Context, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateInitCalls = append(s.CreateInitCalls, CreateInitiativeCall{Input: input})
	if s.CreateInitErr != nil {
		return nil, s.CreateInitErr
	}
	return s.Initiative, s.Err
}
