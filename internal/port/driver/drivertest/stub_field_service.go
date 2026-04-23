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

type StubFieldService struct {
	Fields []domain.Field
	Err    error

	mu              sync.Mutex
	ListFieldsCalls []FieldListCall
}

func (s *StubFieldService) ListFields(_ context.Context, backend string) ([]domain.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListFieldsCalls = append(s.ListFieldsCalls, FieldListCall{Backend: backend})
	return s.Fields, s.Err
}
