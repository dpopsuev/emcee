package driven

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
)

// Ledger is the outbound port for artifact record persistence.
type Ledger interface {
	Put(ctx context.Context, record domain.ArtifactRecord) error
	Get(ctx context.Context, ref string) (*domain.ArtifactRecord, error)
	List(ctx context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error)
	Search(ctx context.Context, query string, limit int) ([]domain.ArtifactRecord, error)
	Similar(ctx context.Context, ref string, limit int) ([]domain.ArtifactRecord, error)
	Stats(ctx context.Context) (*domain.LedgerStats, error)
}
