package stub

import (
	"context"

	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.GistService = (*StubGistService)(nil)

type StubGistService struct {
	GistID  string
	GistURL string
	Err     error
}

func (s *StubGistService) CreateGist(_ context.Context, _, _, _ string, _ bool) (id, url string, err error) {
	return s.GistID, s.GistURL, s.Err
}

func (s *StubGistService) UpdateGist(_ context.Context, _, _, _, _ string) (string, error) {
	return s.GistURL, s.Err
}
