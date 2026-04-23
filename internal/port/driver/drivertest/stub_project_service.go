package drivertest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.ProjectService = (*StubProjectService)(nil)

type ProjListCall struct {
	Backend string
	Filter  domain.ProjectListFilter
}

type ProjCreateCall struct {
	Backend string
	Input   domain.ProjectCreateInput
}

type ProjUpdateCall struct {
	Backend string
	ID      string
	Input   domain.ProjectUpdateInput
}

type StubProjectService struct {
	Projects []domain.Project
	Project  *domain.Project
	Err      error

	mu              sync.Mutex
	ListProjCalls   []ProjListCall
	CreateProjCalls []ProjCreateCall
	UpdateProjCalls []ProjUpdateCall
}

func (s *StubProjectService) ListProjects(_ context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListProjCalls = append(s.ListProjCalls, ProjListCall{Backend: backend, Filter: filter})
	return s.Projects, s.Err
}

func (s *StubProjectService) CreateProject(_ context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateProjCalls = append(s.CreateProjCalls, ProjCreateCall{Backend: backend, Input: input})
	return s.Project, s.Err
}

func (s *StubProjectService) UpdateProject(_ context.Context, backend, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateProjCalls = append(s.UpdateProjCalls, ProjUpdateCall{Backend: backend, ID: id, Input: input})
	return s.Project, s.Err
}
