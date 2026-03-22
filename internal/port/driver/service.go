// Package driver defines inbound ports — interfaces that the application exposes
// to driver adapters (CLI, MCP server). These are the "primary" ports in hexagonal architecture.
package driver

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
)

// IssueService is the inbound port for issue operations.
type IssueService interface {
	List(ctx context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error)
	Get(ctx context.Context, ref string) (*domain.Issue, error)
	Create(ctx context.Context, backend string, input domain.CreateInput) (*domain.Issue, error)
	Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error)
	Search(ctx context.Context, backend string, query string, limit int) ([]domain.Issue, error)
	ListChildren(ctx context.Context, ref string) ([]domain.Issue, error)
	Backends() []string
}

// DocumentService is the inbound port for document operations.
type DocumentService interface {
	ListDocuments(ctx context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error)
	CreateDocument(ctx context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error)
}

// ProjectService is the inbound port for project operations.
type ProjectService interface {
	ListProjects(ctx context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error)
	CreateProject(ctx context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error)
	UpdateProject(ctx context.Context, backend string, id string, input domain.ProjectUpdateInput) (*domain.Project, error)
}

// InitiativeService is the inbound port for initiative operations.
type InitiativeService interface {
	ListInitiatives(ctx context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error)
	CreateInitiative(ctx context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error)
}

// LabelService is the inbound port for label operations.
type LabelService interface {
	ListLabels(ctx context.Context, backend string) ([]domain.Label, error)
	CreateLabel(ctx context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error)
}

// BulkService is the inbound port for bulk issue operations.
// The service handles chunking into backend-appropriate batch sizes.
type BulkService interface {
	BulkCreateIssues(ctx context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error)
	BulkUpdateIssues(ctx context.Context, backend string, inputs []domain.BulkUpdateInput) (*domain.BulkUpdateResult, error)
}
