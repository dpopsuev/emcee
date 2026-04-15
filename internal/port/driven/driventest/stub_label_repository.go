//nolint:dupl // stub repositories share patterns by design
package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.LabelRepository = (*StubLabelRepository)(nil)

type ListLabelsCall struct{}

type CreateLabelCall struct {
	Input domain.LabelCreateInput
}

type StubLabelRepository struct {
	NameVal string
	Labels  []domain.Label
	Label   *domain.Label
	Err     error

	ListLabelsErr  error
	CreateLabelErr error

	mu               sync.Mutex
	ListLabelsCalls  []ListLabelsCall
	CreateLabelCalls []CreateLabelCall
}

func (s *StubLabelRepository) Name() string { return s.NameVal }

func (s *StubLabelRepository) ListLabels(_ context.Context) ([]domain.Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListLabelsCalls = append(s.ListLabelsCalls, ListLabelsCall{})
	if s.ListLabelsErr != nil {
		return nil, s.ListLabelsErr
	}
	return s.Labels, s.Err
}

func (s *StubLabelRepository) CreateLabel(_ context.Context, input domain.LabelCreateInput) (*domain.Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateLabelCalls = append(s.CreateLabelCalls, CreateLabelCall{Input: input})
	if s.CreateLabelErr != nil {
		return nil, s.CreateLabelErr
	}
	return s.Label, s.Err
}
