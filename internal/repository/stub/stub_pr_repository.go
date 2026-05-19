package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	repository "github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.PRRepository = (*StubPRRepository)(nil)

type ListPRsCall struct {
	Filter domain.PRFilter
}

type StubPRRepository struct {
	NameVal string
	PRs     []domain.PullRequest
	Err     error

	mu          sync.Mutex
	ListPRCalls []ListPRsCall
}

func (s *StubPRRepository) Name() string { return s.NameVal }

func (s *StubPRRepository) ListPRs(_ context.Context, filter domain.PRFilter) ([]domain.PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListPRCalls = append(s.ListPRCalls, ListPRsCall{Filter: filter})
	return s.PRs, s.Err
}
