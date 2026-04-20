package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.Ledger = (*StubLedger)(nil)

type LedgerPutCall struct {
	Record domain.ArtifactRecord
}

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

type StubLedger struct {
	Record         *domain.ArtifactRecord
	Records        []domain.ArtifactRecord
	SearchRecords  []domain.ArtifactRecord
	SimilarRecords []domain.ArtifactRecord
	StatsResult    *domain.LedgerStats
	Err            error

	mu           sync.Mutex
	PutCalls     []LedgerPutCall
	GetCalls     []LedgerGetCall
	ListCalls    []LedgerListCall
	SearchCalls  []LedgerSearchCall
	SimilarCalls []LedgerSimilarCall
	StatsCalls   int
}

func (s *StubLedger) Put(_ context.Context, record domain.ArtifactRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PutCalls = append(s.PutCalls, LedgerPutCall{Record: record})
	return s.Err
}

func (s *StubLedger) Get(_ context.Context, ref string) (*domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetCalls = append(s.GetCalls, LedgerGetCall{Ref: ref})
	return s.Record, s.Err
}

func (s *StubLedger) List(_ context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListCalls = append(s.ListCalls, LedgerListCall{Filter: filter})
	return s.Records, s.Err
}

func (s *StubLedger) Search(_ context.Context, query string, limit int) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SearchCalls = append(s.SearchCalls, LedgerSearchCall{Query: query, Limit: limit})
	return s.SearchRecords, s.Err
}

func (s *StubLedger) Similar(_ context.Context, ref string, limit int) ([]domain.ArtifactRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SimilarCalls = append(s.SimilarCalls, LedgerSimilarCall{Ref: ref, Limit: limit})
	return s.SimilarRecords, s.Err
}

func (s *StubLedger) Stats(_ context.Context) (*domain.LedgerStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StatsCalls++
	return s.StatsResult, s.Err
}
