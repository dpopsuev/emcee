package drivertest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.FieldService = (*StubFieldService)(nil)

type FieldListCall struct {
	Backend string
}

type DiscoverFieldsCall struct {
	Backend   string
	ConfigDir string
}

type StubFieldService struct {
	Fields   []domain.Field
	Mappings map[string]string
	Err      error

	mu                  sync.Mutex
	ListFieldsCalls     []FieldListCall
	DiscoverFieldsCalls []DiscoverFieldsCall
}

func (s *StubFieldService) ListFields(_ context.Context, backend string) ([]domain.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListFieldsCalls = append(s.ListFieldsCalls, FieldListCall{Backend: backend})
	return s.Fields, s.Err
}

func (s *StubFieldService) DiscoverFields(_ context.Context, backend, configDir string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DiscoverFieldsCalls = append(s.DiscoverFieldsCalls, DiscoverFieldsCall{Backend: backend, ConfigDir: configDir})
	return s.Mappings, s.Err
}
