package driventest

import (
	"context"
	"fmt"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

type mockRepo struct {
	issues []domain.Issue
}

func (m *mockRepo) Name() string { return "mock" }

func (m *mockRepo) List(_ context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	out := make([]domain.Issue, len(m.issues))
	copy(out, m.issues)
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) Get(_ context.Context, key string) (*domain.Issue, error) {
	for _, i := range m.issues {
		if i.Key == key {
			return &i, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockRepo) Create(_ context.Context, input domain.CreateInput) (*domain.Issue, error) {
	issue := domain.Issue{
		Ref:   "mock:NEW-1",
		ID:    "new-id",
		Key:   "NEW-1",
		Title: input.Title,
	}
	m.issues = append(m.issues, issue)
	return &issue, nil
}

func (m *mockRepo) Update(_ context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	for i, issue := range m.issues {
		if issue.Key == key {
			if input.Title != nil {
				m.issues[i].Title = *input.Title
			}
			return &m.issues[i], nil
		}
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockRepo) Search(_ context.Context, query string, limit int) ([]domain.Issue, error) {
	return m.issues, nil
}

func (m *mockRepo) ListChildren(_ context.Context, key string) ([]domain.Issue, error) {
	return nil, nil
}

func TestMockRepoContract(t *testing.T) {
	RunContractTests(t, func(t *testing.T) (driven.IssueRepository, string) {
		return &mockRepo{
			issues: []domain.Issue{
				{Ref: "mock:M-1", ID: "id-1", Key: "M-1", Title: "First", Status: domain.StatusTodo},
				{Ref: "mock:M-2", ID: "id-2", Key: "M-2", Title: "Second", Status: domain.StatusDone},
			},
		}, "M-1"
	})
}
