//nolint:dupl // stub services share patterns by design
package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.LabelService = (*StubLabelService)(nil)

type LabelListCall struct {
	Backend string
}

type LabelCreateCall struct {
	Backend string
	Input   domain.LabelCreateInput
}

type StubLabelService struct {
	Labels []domain.Label
	Label  *domain.Label
	Err    error

	mu               sync.Mutex
	ListLabelsCalls  []LabelListCall
	CreateLabelCalls []LabelCreateCall
}

func (s *StubLabelService) ListLabels(_ context.Context, backend string) ([]domain.Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListLabelsCalls = append(s.ListLabelsCalls, LabelListCall{Backend: backend})
	return s.Labels, s.Err
}

func (s *StubLabelService) CreateLabel(_ context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateLabelCalls = append(s.CreateLabelCalls, LabelCreateCall{Backend: backend, Input: input})
	return s.Label, s.Err
}
