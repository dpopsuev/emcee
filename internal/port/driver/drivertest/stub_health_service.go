package drivertest

import (
	"sync"

	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.HealthService = (*StubHealthService)(nil)

type StubHealthService struct {
	Status *driver.HealthStatus

	mu    sync.Mutex
	Calls int
}

func (s *StubHealthService) Health() *driver.HealthStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls++
	return s.Status
}
