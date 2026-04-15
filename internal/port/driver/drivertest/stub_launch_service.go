package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.LaunchService = (*StubLaunchService)(nil)

type LaunchListCall struct {
	Backend string
	Filter  domain.LaunchFilter
}

type LaunchGetCall struct {
	Backend string
	ID      string
}

type TestItemsListCall struct {
	Backend  string
	LaunchID string
	Filter   domain.TestItemFilter
}

type TestItemGetCall struct {
	Backend string
	ID      string
}

type DefectUpdateCall struct {
	Backend string
	Updates []domain.DefectUpdate
}

type StubLaunchService struct {
	Launches  []domain.Launch
	Launch    *domain.Launch
	TestItems []domain.TestItem
	TestItem  *domain.TestItem
	Err       error

	mu                 sync.Mutex
	ListLaunchesCalls  []LaunchListCall
	GetLaunchCalls     []LaunchGetCall
	ListTestItemsCalls []TestItemsListCall
	GetTestItemCalls   []TestItemGetCall
	UpdateDefectsCalls []DefectUpdateCall
}

func (s *StubLaunchService) ListLaunches(_ context.Context, backend string, filter domain.LaunchFilter) ([]domain.Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListLaunchesCalls = append(s.ListLaunchesCalls, LaunchListCall{Backend: backend, Filter: filter})
	return s.Launches, s.Err
}

func (s *StubLaunchService) GetLaunch(_ context.Context, backend, id string) (*domain.Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLaunchCalls = append(s.GetLaunchCalls, LaunchGetCall{Backend: backend, ID: id})
	return s.Launch, s.Err
}

func (s *StubLaunchService) ListTestItems(_ context.Context, backend, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListTestItemsCalls = append(s.ListTestItemsCalls, TestItemsListCall{Backend: backend, LaunchID: launchID, Filter: filter})
	return s.TestItems, s.Err
}

func (s *StubLaunchService) GetTestItem(_ context.Context, backend, id string) (*domain.TestItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetTestItemCalls = append(s.GetTestItemCalls, TestItemGetCall{Backend: backend, ID: id})
	return s.TestItem, s.Err
}

func (s *StubLaunchService) UpdateDefects(_ context.Context, backend string, updates []domain.DefectUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdateDefectsCalls = append(s.UpdateDefectsCalls, DefectUpdateCall{Backend: backend, Updates: updates})
	return s.Err
}
