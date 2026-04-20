package triage

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.Ledger = (*InMemoryLedger)(nil)

// ErrLedgerNotFound is returned when a ref is not in the ledger.
var ErrLedgerNotFound = errors.New("ledger: record not found")

// InMemoryLedger is a mutex-protected in-memory ledger.
type InMemoryLedger struct {
	mu      sync.RWMutex
	records map[string]domain.ArtifactRecord
}

// NewInMemoryLedger creates an empty ledger.
func NewInMemoryLedger() *InMemoryLedger {
	return &InMemoryLedger{
		records: make(map[string]domain.ArtifactRecord),
	}
}

// Put upserts a record by ref.
func (l *InMemoryLedger) Put(_ context.Context, record domain.ArtifactRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records[record.Ref] = record
	return nil
}

// Get returns a record by ref.
func (l *InMemoryLedger) Get(_ context.Context, ref string) (*domain.ArtifactRecord, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	r, ok := l.records[ref]
	if !ok {
		return nil, ErrLedgerNotFound
	}
	return &r, nil
}

// List returns records matching the filter.
func (l *InMemoryLedger) List(_ context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	results := make([]domain.ArtifactRecord, 0, len(l.records))
	for ref := range l.records {
		r := l.records[ref]
		if filter.Backend != "" && r.Backend != filter.Backend {
			continue
		}
		if filter.Type != "" && r.Type != filter.Type {
			continue
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		if filter.Component != "" && !containsStr(r.Components, filter.Component) {
			continue
		}
		results = append(results, r)
		if filter.Limit > 0 && len(results) >= filter.Limit {
			break
		}
	}
	return results, nil
}

// Search does a simple substring match across title and text fields.
func (l *InMemoryLedger) Search(_ context.Context, query string, limit int) ([]domain.ArtifactRecord, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	q := strings.ToLower(query)
	var results []domain.ArtifactRecord
	for ref := range l.records {
		r := l.records[ref]
		if strings.Contains(strings.ToLower(r.Title), q) || strings.Contains(strings.ToLower(r.Text), q) {
			results = append(results, r)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Stats returns aggregate counts.
func (l *InMemoryLedger) Stats(_ context.Context) (*domain.LedgerStats, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := &domain.LedgerStats{
		ByBackend: make(map[string]int),
	}
	for ref := range l.records {
		stats.Total++
		stats.ByBackend[l.records[ref].Backend]++
	}
	return stats, nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
