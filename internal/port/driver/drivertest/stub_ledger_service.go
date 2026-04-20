package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.LedgerService = (*StubLedgerService)(nil)

type LedgerGetCall struct {
	Ref string
}

type LedgerListCall struct {
	Filter domain.LedgerFilter
}

type StubLedgerService struct {
	Record      *domain.ArtifactRecord
	Records     []domain.ArtifactRecord
	StatsResult *domain.LedgerStats
	Err         error

	mu               sync.Mutex
	LedgerGetCalls   []LedgerGetCall
	LedgerListCalls  []LedgerListCall
	LedgerStatsCalls int
}

func (s *StubLedgerService) LedgerGet(_ context.Context, ref string) (*domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerGetCalls = append(s.LedgerGetCalls, LedgerGetCall{Ref: ref})
	return s.Record, s.Err
}

func (s *StubLedgerService) LedgerList(_ context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerListCalls = append(s.LedgerListCalls, LedgerListCall{Filter: filter})
	return s.Records, s.Err
}

func (s *StubLedgerService) LedgerStats(_ context.Context) (*domain.LedgerStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerStatsCalls++
	return s.StatsResult, s.Err
}
