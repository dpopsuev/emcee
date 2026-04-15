//nolint:dupl // stub repositories share patterns by design
package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.DocumentRepository = (*StubDocumentRepository)(nil)

type ListDocumentsCall struct {
	Filter domain.DocumentListFilter
}

type CreateDocumentCall struct {
	Input domain.DocumentCreateInput
}

type StubDocumentRepository struct {
	NameVal   string
	Documents []domain.Document
	Document  *domain.Document
	Err       error

	ListDocsErr  error
	CreateDocErr error

	mu             sync.Mutex
	ListDocsCalls  []ListDocumentsCall
	CreateDocCalls []CreateDocumentCall
}

func (s *StubDocumentRepository) Name() string { return s.NameVal }

func (s *StubDocumentRepository) ListDocuments(_ context.Context, filter domain.DocumentListFilter) ([]domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListDocsCalls = append(s.ListDocsCalls, ListDocumentsCall{Filter: filter})
	if s.ListDocsErr != nil {
		return nil, s.ListDocsErr
	}
	return s.Documents, s.Err
}

func (s *StubDocumentRepository) CreateDocument(_ context.Context, input domain.DocumentCreateInput) (*domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateDocCalls = append(s.CreateDocCalls, CreateDocumentCall{Input: input})
	if s.CreateDocErr != nil {
		return nil, s.CreateDocErr
	}
	return s.Document, s.Err
}
