package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.StatusService = (*StubStatusService)(nil)

type StubStatusService struct {
	Mappings map[string]string
	Err      error

	mu                    sync.Mutex
	DiscoverStatusesCalls int
}

func (s *StubStatusService) DiscoverStatuses(_ context.Context, _, _ string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DiscoverStatusesCalls++
	return s.Mappings, s.Err
}
