//nolint:dupl // stub services share patterns by design
package drivertest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.DocumentService = (*StubDocumentService)(nil)

type DocListCall struct {
	Backend string
	Filter  domain.DocumentListFilter
}

type DocCreateCall struct {
	Backend string
	Input   domain.DocumentCreateInput
}

type StubDocumentService struct {
	Documents []domain.Document
	Document  *domain.Document
	Err       error

	mu             sync.Mutex
	ListDocsCalls  []DocListCall
	CreateDocCalls []DocCreateCall
}

func (s *StubDocumentService) ListDocuments(_ context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListDocsCalls = append(s.ListDocsCalls, DocListCall{Backend: backend, Filter: filter})
	return s.Documents, s.Err
}

func (s *StubDocumentService) CreateDocument(_ context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateDocCalls = append(s.CreateDocCalls, DocCreateCall{Backend: backend, Input: input})
	return s.Document, s.Err
}
