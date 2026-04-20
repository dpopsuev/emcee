package driven

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
)

// Ledger is the outbound port for artifact record persistence.
type Ledger interface {
	Put(ctx context.Context, record domain.ArtifactRecord) error
	Get(ctx context.Context, ref string) (*domain.ArtifactRecord, error)
	List(ctx context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error)
	Stats(ctx context.Context) (*domain.LedgerStats, error)
}
