// Package driver defines inbound ports — interfaces that the application exposes
// to driver adapters (CLI, MCP server). These are the "primary" ports in hexagonal architecture.
package driver

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
)

// IssueService is the inbound port for issue operations.
// The CLI and MCP server call this interface; the application service implements it.
type IssueService interface {
	// List issues from a specific backend.
	List(ctx context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error)

	// Get a single issue by canonical ref (e.g. "linear:HEG-17").
	Get(ctx context.Context, ref string) (*domain.Issue, error)

	// Create an issue on a specific backend.
	Create(ctx context.Context, backend string, input domain.CreateInput) (*domain.Issue, error)

	// Update an issue by canonical ref.
	Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error)

	// Search across a specific backend.
	Search(ctx context.Context, backend string, query string, limit int) ([]domain.Issue, error)

	// Backends returns the names of all registered backends.
	Backends() []string
}
