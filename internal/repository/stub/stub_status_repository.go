package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.StatusRepository = (*StubStatusRepository)(nil)

type StubStatusRepository struct {
	NameVal  string
	Statuses []domain.StatusEntry
	Err      error

	mu              sync.Mutex
	ListStatusCalls int
}

func (s *StubStatusRepository) Name() string { return s.NameVal }

func (s *StubStatusRepository) ListStatuses(_ context.Context) ([]domain.StatusEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListStatusCalls++
	return s.Statuses, s.Err
}
