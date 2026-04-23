package driventest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
)

var _ driven.JQLRepository = (*StubJQLRepository)(nil)

type SearchJQLCall struct {
	JQL   string
	Limit int
}

type StubJQLRepository struct {
	NameVal string
	Issues  []domain.Issue
	Err     error

	mu             sync.Mutex
	SearchJQLCalls []SearchJQLCall
}

func (s *StubJQLRepository) Name() string { return s.NameVal }

func (s *StubJQLRepository) SearchJQL(_ context.Context, jql string, limit int) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SearchJQLCalls = append(s.SearchJQLCalls, SearchJQLCall{JQL: jql, Limit: limit})
	return s.Issues, s.Err
}
