// Package driven defines outbound ports — interfaces that driven adapters must implement.
// These are the "secondary" ports in hexagonal architecture: the application
// depends on these interfaces, and adapters (Linear, GitHub, Jira) provide implementations.
package driven

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
)

// IssueRepository is the outbound port for issue persistence/retrieval.
// Each backend (Linear, GitHub, Jira) implements this interface.
type IssueRepository interface {
	// Name returns the backend identifier (e.g. "linear", "github", "jira").
	Name() string

	List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error)
	Get(ctx context.Context, key string) (*domain.Issue, error)
	Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error)
	Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error)
	Search(ctx context.Context, query string, limit int) ([]domain.Issue, error)
	ListChildren(ctx context.Context, key string) ([]domain.Issue, error)
}

// DocumentRepository is the outbound port for document operations.
type DocumentRepository interface {
	Name() string
	ListDocuments(ctx context.Context, filter domain.DocumentListFilter) ([]domain.Document, error)
	CreateDocument(ctx context.Context, input domain.DocumentCreateInput) (*domain.Document, error)
}

// ProjectRepository is the outbound port for project operations.
type ProjectRepository interface {
	Name() string
	ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error)
	CreateProject(ctx context.Context, input domain.ProjectCreateInput) (*domain.Project, error)
	UpdateProject(ctx context.Context, id string, input domain.ProjectUpdateInput) (*domain.Project, error)
}

// InitiativeRepository is the outbound port for initiative operations.
type InitiativeRepository interface {
	Name() string
	ListInitiatives(ctx context.Context, filter domain.InitiativeListFilter) ([]domain.Initiative, error)
	CreateInitiative(ctx context.Context, input domain.InitiativeCreateInput) (*domain.Initiative, error)
}

// LabelRepository is the outbound port for label operations.
type LabelRepository interface {
	Name() string
	ListLabels(ctx context.Context) ([]domain.Label, error)
	CreateLabel(ctx context.Context, input domain.LabelCreateInput) (*domain.Label, error)
}

// BulkIssueRepository is the outbound port for batch issue creation.
// Implementations handle at most 50 issues per call.
type BulkIssueRepository interface {
	Name() string
	BulkCreateIssues(ctx context.Context, inputs []domain.CreateInput) ([]domain.Issue, error)
}

// CommentRepository is the outbound port for issue comment operations.
type CommentRepository interface {
	Name() string
	ListComments(ctx context.Context, key string) ([]domain.Comment, error)
	AddComment(ctx context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error)
}

// ExternalLinkRepository is the outbound port for remote link retrieval (PRs, commits).
type ExternalLinkRepository interface {
	Name() string
	ListExternalLinks(ctx context.Context, key string) ([]domain.ExternalLink, error)
}

// IssueLinkRepository is the outbound port for creating issue-to-issue links.
type IssueLinkRepository interface {
	Name() string
	CreateIssueLink(ctx context.Context, input domain.IssueLinkInput) error
}

// FieldRepository is the outbound port for field metadata discovery.
type FieldRepository interface {
	Name() string
	ListFields(ctx context.Context) ([]domain.Field, error)
}

// JQLRepository is the outbound port for raw JQL query passthrough (Jira-specific).
type JQLRepository interface {
	Name() string
	SearchJQL(ctx context.Context, jql string, limit int) ([]domain.Issue, error)
}

// PRRepository is the outbound port for pull request / merge request operations.
type PRRepository interface {
	Name() string
	ListPRs(ctx context.Context, filter domain.PRFilter) ([]domain.PullRequest, error)
}

// UserResolver resolves human-readable names to backend-specific IDs.
type UserResolver interface {
	ResolveUser(ctx context.Context, name string) (string, error)
}
