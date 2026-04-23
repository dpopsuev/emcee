package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.IssueRepository = (*StubIssueRepository)(nil)

// Call types for spy recording.

type ListCall struct {
	Filter domain.ListFilter
}

type GetCall struct {
	Key string
}

type CreateCall struct {
	Input domain.CreateInput
}

type UpdateCall struct {
	Key   string
	Input domain.UpdateInput
}

type SearchCall struct {
	Query string
	Limit int
}

type ListChildrenCall struct {
	Key string
}

// StubIssueRepository is a Battery-style stub with spy recording
// and error injection for the driven.IssueRepository interface.
type StubIssueRepository struct {
	NameVal string
	Issues  []domain.Issue
	Issue   *domain.Issue
	Err     error

	// Per-method error overrides. If set, take precedence over Err.
	ListErr     error
	GetErr      error
	CreateErr   error
	UpdateErr   error
	SearchErr   error
	ChildrenErr error

	mu            sync.Mutex
	ListCalls     []ListCall
	GetCalls      []GetCall
	CreateCalls   []CreateCall
	UpdateCalls   []UpdateCall
	SearchCalls   []SearchCall
	ChildrenCalls []ListChildrenCall
}

func NewStubIssueRepository(name string) *StubIssueRepository {
	return &StubIssueRepository{NameVal: name}
}

func (s *StubIssueRepository) Name() string { return s.NameVal }

func (s *StubIssueRepository) List(_ context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListCalls = append(s.ListCalls, ListCall{Filter: filter})
	if s.ListErr != nil {
		return nil, s.ListErr
	}
	return s.Issues, s.Err
}

func (s *StubIssueRepository) Get(_ context.Context, key string) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetCalls = append(s.GetCalls, GetCall{Key: key})
	if s.GetErr != nil {
		return nil, s.GetErr
	}
	return s.Issue, s.Err
}

func (s *StubIssueRepository) Create(_ context.Context, input domain.CreateInput) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateCalls = append(s.CreateCalls, CreateCall{Input: input})
	if s.CreateErr != nil {
		return nil, s.CreateErr
	}
	return s.Issue, s.Err
}

func (s *StubIssueRepository) Update(_ context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateCalls = append(s.UpdateCalls, UpdateCall{Key: key, Input: input})
	if s.UpdateErr != nil {
		return nil, s.UpdateErr
	}
	return s.Issue, s.Err
}

func (s *StubIssueRepository) Search(_ context.Context, query string, limit int) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SearchCalls = append(s.SearchCalls, SearchCall{Query: query, Limit: limit})
	if s.SearchErr != nil {
		return nil, s.SearchErr
	}
	return s.Issues, s.Err
}

func (s *StubIssueRepository) ListChildren(_ context.Context, key string) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChildrenCalls = append(s.ChildrenCalls, ListChildrenCall{Key: key})
	if s.ChildrenErr != nil {
		return nil, s.ChildrenErr
	}
	return s.Issues, s.Err
}
