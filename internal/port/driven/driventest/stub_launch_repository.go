//nolint:dupl // stub repositories share patterns by design
package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.LaunchRepository = (*StubLaunchRepository)(nil)

type ListLaunchesCall struct {
	Filter domain.LaunchFilter
}

type GetLaunchCall struct {
	ID string
}

type ListTestItemsCall struct {
	LaunchID string
	Filter   domain.TestItemFilter
}

type GetTestItemCall struct {
	ID string
}

type UpdateDefectsCall struct {
	Updates []domain.DefectUpdate
}

type StubLaunchRepository struct {
	NameVal   string
	Launches  []domain.Launch
	Launch    *domain.Launch
	TestItems []domain.TestItem
	TestItem  *domain.TestItem
	Err       error

	mu                 sync.Mutex
	ListLaunchesCalls  []ListLaunchesCall
	GetLaunchCalls     []GetLaunchCall
	ListTestItemsCalls []ListTestItemsCall
	GetTestItemCalls   []GetTestItemCall
	UpdateDefectsCalls []UpdateDefectsCall
}

func (s *StubLaunchRepository) Name() string { return s.NameVal }

func (s *StubLaunchRepository) ListLaunches(_ context.Context, filter domain.LaunchFilter) ([]domain.Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListLaunchesCalls = append(s.ListLaunchesCalls, ListLaunchesCall{Filter: filter})
	return s.Launches, s.Err
}

func (s *StubLaunchRepository) GetLaunch(_ context.Context, id string) (*domain.Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLaunchCalls = append(s.GetLaunchCalls, GetLaunchCall{ID: id})
	return s.Launch, s.Err
}

func (s *StubLaunchRepository) ListTestItems(_ context.Context, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListTestItemsCalls = append(s.ListTestItemsCalls, ListTestItemsCall{LaunchID: launchID, Filter: filter})
	return s.TestItems, s.Err
}

func (s *StubLaunchRepository) GetTestItem(_ context.Context, id string) (*domain.TestItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetTestItemCalls = append(s.GetTestItemCalls, GetTestItemCall{ID: id})
	return s.TestItem, s.Err
}

func (s *StubLaunchRepository) UpdateDefects(_ context.Context, updates []domain.DefectUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateDefectsCalls = append(s.UpdateDefectsCalls, UpdateDefectsCall{Updates: updates})
	return s.Err
}
