package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.ProjectRepository = (*StubProjectRepository)(nil)

type ListProjectsCall struct {
	Filter domain.ProjectListFilter
}

type CreateProjectCall struct {
	Input domain.ProjectCreateInput
}

type UpdateProjectCall struct {
	ID    string
	Input domain.ProjectUpdateInput
}

type StubProjectRepository struct {
	NameVal  string
	Projects []domain.Project
	Project  *domain.Project
	Err      error

	ListProjErr   error
	CreateProjErr error
	UpdateProjErr error

	mu              sync.Mutex
	ListProjCalls   []ListProjectsCall
	CreateProjCalls []CreateProjectCall
	UpdateProjCalls []UpdateProjectCall
}

func (s *StubProjectRepository) Name() string { return s.NameVal }

func (s *StubProjectRepository) ListProjects(_ context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListProjCalls = append(s.ListProjCalls, ListProjectsCall{Filter: filter})
	if s.ListProjErr != nil {
		return nil, s.ListProjErr
	}
	return s.Projects, s.Err
}

func (s *StubProjectRepository) CreateProject(_ context.Context, input domain.ProjectCreateInput) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateProjCalls = append(s.CreateProjCalls, CreateProjectCall{Input: input})
	if s.CreateProjErr != nil {
		return nil, s.CreateProjErr
	}
	return s.Project, s.Err
}

func (s *StubProjectRepository) UpdateProject(_ context.Context, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateProjCalls = append(s.UpdateProjCalls, UpdateProjectCall{ID: id, Input: input})
	if s.UpdateProjErr != nil {
		return nil, s.UpdateProjErr
	}
	return s.Project, s.Err
}
