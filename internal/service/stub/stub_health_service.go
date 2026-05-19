package stub

import (
	"sync"

	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.HealthService = (*StubHealthService)(nil)

type StubHealthService struct {
	Status *service.HealthStatus

	mu    sync.Mutex
	Calls int
}

func (s *StubHealthService) Health() *service.HealthStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls++
	return s.Status
}
