package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.ChangelogService = (*StubChangelogService)(nil)

type ListChangelogCall struct {
	Ref   string
	Limit int
}

type StubChangelogService struct {
	Entries []domain.ChangelogEntry
	Err     error

	mu                 sync.Mutex
	ListChangelogCalls []ListChangelogCall
}

func (s *StubChangelogService) ListChangelog(_ context.Context, ref string, limit int) ([]domain.ChangelogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListChangelogCalls = append(s.ListChangelogCalls, ListChangelogCall{Ref: ref, Limit: limit})
	return s.Entries, s.Err
}
