// Package app contains the application service — the hexagon's core orchestration layer.
// It implements the driver (inbound) port and delegates to driven (outbound) adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

const batchSize = 50

var (
	ErrUnknownBackend  = errors.New("unknown backend")
	ErrInvalidRef      = errors.New("invalid ref")
	ErrNotSupported    = errors.New("operation not supported by backend")
)

// Service implements all driver port interfaces by routing to the appropriate repository.
type Service struct {
	repos      map[string]driven.IssueRepository
	docRepos   map[string]driven.DocumentRepository
	projRepos  map[string]driven.ProjectRepository
	initRepos  map[string]driven.InitiativeRepository
	labelRepos map[string]driven.LabelRepository
	bulkRepos  map[string]driven.BulkIssueRepository
}

// NewService creates the application service with the given repositories.
// Repositories that implement additional interfaces (DocumentRepository, etc.)
// are automatically registered for those capabilities.
func NewService(repos ...driven.IssueRepository) *Service {
	s := &Service{
		repos:      make(map[string]driven.IssueRepository, len(repos)),
		docRepos:   make(map[string]driven.DocumentRepository),
		projRepos:  make(map[string]driven.ProjectRepository),
		initRepos:  make(map[string]driven.InitiativeRepository),
		labelRepos: make(map[string]driven.LabelRepository),
		bulkRepos:  make(map[string]driven.BulkIssueRepository),
	}
	for _, r := range repos {
		name := r.Name()
		s.repos[name] = r
		if dr, ok := r.(driven.DocumentRepository); ok {
			s.docRepos[name] = dr
		}
		if pr, ok := r.(driven.ProjectRepository); ok {
			s.projRepos[name] = pr
		}
		if ir, ok := r.(driven.InitiativeRepository); ok {
			s.initRepos[name] = ir
		}
		if lr, ok := r.(driven.LabelRepository); ok {
			s.labelRepos[name] = lr
		}
		if br, ok := r.(driven.BulkIssueRepository); ok {
			s.bulkRepos[name] = br
		}
	}
	return s
}

// ParseRef splits "linear:HEG-17" into backend and key.
func ParseRef(ref string) (backend, key string, err error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %q (expected backend:key, e.g. linear:HEG-17)", ErrInvalidRef, ref)
	}
	return parts[0], parts[1], nil
}

func (s *Service) repo(name string) (driven.IssueRepository, error) {
	r, ok := s.repos[name]
	if !ok {
		return nil, s.unknownBackendErr(name)
	}
	return r, nil
}

func (s *Service) unknownBackendErr(name string) error {
	available := make([]string, 0, len(s.repos))
	for k := range s.repos {
		available = append(available, k)
	}
	return fmt.Errorf("%w: %q (available: %s)", ErrUnknownBackend, name, strings.Join(available, ", "))
}

func (s *Service) notSupportedErr(backend, op string) error {
	return fmt.Errorf("%w: %q does not support %s", ErrNotSupported, backend, op)
}

// --- Issue operations ---

func (s *Service) List(ctx context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, ref string) (*domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, key)
}

func (s *Service) Create(ctx context.Context, backend string, input domain.CreateInput) (*domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Create(ctx, input)
}

func (s *Service) Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Update(ctx, key, input)
}

func (s *Service) Search(ctx context.Context, backend string, query string, limit int) ([]domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Search(ctx, query, limit)
}

func (s *Service) ListChildren(ctx context.Context, ref string) ([]domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.ListChildren(ctx, key)
}

func (s *Service) Backends() []string {
	names := make([]string, 0, len(s.repos))
	for k := range s.repos {
		names = append(names, k)
	}
	return names
}

// --- Document operations ---

func (s *Service) ListDocuments(ctx context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error) {
	r, ok := s.docRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "documents")
	}
	return r.ListDocuments(ctx, filter)
}

func (s *Service) CreateDocument(ctx context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error) {
	r, ok := s.docRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "documents")
	}
	return r.CreateDocument(ctx, input)
}

// --- Project operations ---

func (s *Service) ListProjects(ctx context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error) {
	r, ok := s.projRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "projects")
	}
	return r.ListProjects(ctx, filter)
}

func (s *Service) CreateProject(ctx context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error) {
	r, ok := s.projRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "projects")
	}
	return r.CreateProject(ctx, input)
}

// --- Initiative operations ---

func (s *Service) ListInitiatives(ctx context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	r, ok := s.initRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "initiatives")
	}
	return r.ListInitiatives(ctx, filter)
}

func (s *Service) CreateInitiative(ctx context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	r, ok := s.initRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "initiatives")
	}
	return r.CreateInitiative(ctx, input)
}

// --- Label operations ---

func (s *Service) ListLabels(ctx context.Context, backend string) ([]domain.Label, error) {
	r, ok := s.labelRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "labels")
	}
	return r.ListLabels(ctx)
}

func (s *Service) CreateLabel(ctx context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error) {
	r, ok := s.labelRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "labels")
	}
	return r.CreateLabel(ctx, input)
}

// --- Bulk operations ---

func (s *Service) BulkCreateIssues(ctx context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error) {
	r, ok := s.bulkRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "bulk create")
	}

	result := &domain.BulkCreateResult{Total: len(inputs)}

	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		result.Batches++

		created, err := r.BulkCreateIssues(ctx, inputs[i:end])
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("batch %d: %v", result.Batches, err))
			continue
		}
		result.Created = append(result.Created, created...)
	}
	return result, nil
}
