package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.BulkIssueRepository = (*StubBulkIssueRepository)(nil)

type BulkCreateIssuesCall struct {
	Inputs []domain.CreateInput
}

type StubBulkIssueRepository struct {
	NameVal string
	Issues  []domain.Issue
	Err     error

	mu              sync.Mutex
	BulkCreateCalls []BulkCreateIssuesCall
}

func (s *StubBulkIssueRepository) Name() string { return s.NameVal }

func (s *StubBulkIssueRepository) BulkCreateIssues(_ context.Context, inputs []domain.CreateInput) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BulkCreateCalls = append(s.BulkCreateCalls, BulkCreateIssuesCall{Inputs: inputs})
	return s.Issues, s.Err
}
