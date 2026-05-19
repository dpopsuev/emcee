package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	repository "github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.ChangelogRepository = (*StubChangelogRepository)(nil)

type ListChangelogCall struct {
	Key   string
	Limit int
}

type StubChangelogRepository struct {
	NameVal string
	Entries []domain.ChangelogEntry
	Err     error

	mu                 sync.Mutex
	ListChangelogCalls []ListChangelogCall
}

func (s *StubChangelogRepository) Name() string { return s.NameVal }

func (s *StubChangelogRepository) ListChangelog(_ context.Context, key string, limit int) ([]domain.ChangelogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListChangelogCalls = append(s.ListChangelogCalls, ListChangelogCall{Key: key, Limit: limit})
	return s.Entries, s.Err
}
