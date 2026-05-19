package stub

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
	repository "github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.ExternalLinkRepository = (*StubExternalLinkRepository)(nil)

type StubExternalLinkRepository struct {
	NameVal       string
	ExternalLinks []domain.ExternalLink
	Err           error
}

func (s *StubExternalLinkRepository) Name() string { return s.NameVal }

func (s *StubExternalLinkRepository) ListExternalLinks(_ context.Context, _ string) ([]domain.ExternalLink, error) {
	return s.ExternalLinks, s.Err
}
