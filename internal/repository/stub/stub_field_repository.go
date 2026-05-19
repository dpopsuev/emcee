package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	repository "github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.FieldRepository = (*StubFieldRepository)(nil)

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
