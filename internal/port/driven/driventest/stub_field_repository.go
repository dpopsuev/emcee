package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.FieldRepository = (*StubFieldRepository)(nil)

type ListFieldsCall struct{}

type StubFieldRepository struct {
	NameVal string
	Fields  []domain.Field
	Err     error

	mu              sync.Mutex
	ListFieldsCalls []ListFieldsCall
}

func (s *StubFieldRepository) Name() string { return s.NameVal }

func (s *StubFieldRepository) ListFields(_ context.Context) ([]domain.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListFieldsCalls = append(s.ListFieldsCalls, ListFieldsCall{})
	return s.Fields, s.Err
}
