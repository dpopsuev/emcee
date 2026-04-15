package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.IssueService = (*StubIssueService)(nil)

type IssueListCall struct {
	Backend string
	Filter  domain.ListFilter
}

type IssueGetCall struct {
	Ref string
}

type IssueCreateCall struct {
	Backend string
	Input   domain.CreateInput
}

type IssueUpdateCall struct {
	Ref   string
	Input domain.UpdateInput
}

type IssueSearchCall struct {
	Backend string
	Query   string
	Limit   int
}

type IssueChildrenCall struct {
	Ref string
}

type StubIssueService struct {
	Issues      []domain.Issue
	Issue       *domain.Issue
	BackendList []string
	Err         error

	mu            sync.Mutex
	ListCalls     []IssueListCall
	GetCalls      []IssueGetCall
	CreateCalls   []IssueCreateCall
	UpdateCalls   []IssueUpdateCall
	SearchCalls   []IssueSearchCall
	ChildrenCalls []IssueChildrenCall
}

func (s *StubIssueService) List(_ context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListCalls = append(s.ListCalls, IssueListCall{Backend: backend, Filter: filter})
	return s.Issues, s.Err
}

func (s *StubIssueService) Get(_ context.Context, ref string) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetCalls = append(s.GetCalls, IssueGetCall{Ref: ref})
	return s.Issue, s.Err
}

func (s *StubIssueService) Create(_ context.Context, backend string, input domain.CreateInput) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateCalls = append(s.CreateCalls, IssueCreateCall{Backend: backend, Input: input})
	return s.Issue, s.Err
}

func (s *StubIssueService) Update(_ context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateCalls = append(s.UpdateCalls, IssueUpdateCall{Ref: ref, Input: input})
	return s.Issue, s.Err
}

func (s *StubIssueService) Search(_ context.Context, backend, query string, limit int) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SearchCalls = append(s.SearchCalls, IssueSearchCall{Backend: backend, Query: query, Limit: limit})
	return s.Issues, s.Err
}

func (s *StubIssueService) ListChildren(_ context.Context, ref string) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChildrenCalls = append(s.ChildrenCalls, IssueChildrenCall{Ref: ref})
	return s.Issues, s.Err
}

func (s *StubIssueService) Backends() []string {
	return s.BackendList
}
