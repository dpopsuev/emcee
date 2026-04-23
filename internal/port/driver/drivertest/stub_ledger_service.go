package drivertest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.LedgerService = (*StubLedgerService)(nil)

type LedgerGetCall struct {
	Ref string
}

type LedgerListCall struct {
	Filter domain.LedgerFilter
}

type LedgerSearchCall struct {
	Query string
	Limit int
}

type LedgerSimilarCall struct {
	Ref   string
	Limit int
}

type LedgerIngestCall struct {
	Record domain.ArtifactRecord
}

type StubLedgerService struct {
	Record         *domain.ArtifactRecord
	Records        []domain.ArtifactRecord
	SearchRecords  []domain.ArtifactRecord
	SimilarRecords []domain.ArtifactRecord
	StatsResult    *domain.LedgerStats
	Err            error

	mu                 sync.Mutex
	LedgerGetCalls     []LedgerGetCall
	LedgerListCalls    []LedgerListCall
	LedgerSearchCalls  []LedgerSearchCall
	LedgerSimilarCalls []LedgerSimilarCall
	LedgerIngestCalls  []LedgerIngestCall
	LedgerStatsCalls   int
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

func (s *StubLedgerService) LedgerSearch(_ context.Context, query string, limit int) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerSearchCalls = append(s.LedgerSearchCalls, LedgerSearchCall{Query: query, Limit: limit})
	return s.SearchRecords, s.Err
}

func (s *StubLedgerService) LedgerSimilar(_ context.Context, ref string, limit int) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerSimilarCalls = append(s.LedgerSimilarCalls, LedgerSimilarCall{Ref: ref, Limit: limit})
	return s.SimilarRecords, s.Err
}

func (s *StubLedgerService) LedgerIngest(_ context.Context, record domain.ArtifactRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerIngestCalls = append(s.LedgerIngestCalls, LedgerIngestCall{Record: record})
	return s.Err
}

func (s *StubLedgerService) LedgerStats(_ context.Context) (*domain.LedgerStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LedgerStatsCalls++
	return s.StatsResult, s.Err
}
