package driventest

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.ExternalLinkRepository = (*StubExternalLinkRepository)(nil)

type StubExternalLinkRepository struct {
	NameVal       string
	ExternalLinks []domain.ExternalLink
	Err           error
}

func (s *StubExternalLinkRepository) Name() string { return s.NameVal }

func (s *StubExternalLinkRepository) ListExternalLinks(_ context.Context, _ string) ([]domain.ExternalLink, error) {
	return s.ExternalLinks, s.Err
}
