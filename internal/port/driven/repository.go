// Package driven defines outbound ports — interfaces that driven adapters must implement.
// These are the "secondary" ports in hexagonal architecture: the application
// depends on these interfaces, and adapters (Linear, GitHub, Jira) provide implementations.
package driven

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
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
}
