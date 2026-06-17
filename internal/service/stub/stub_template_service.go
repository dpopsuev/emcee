package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.TemplateService = (*StubTemplateService)(nil)

type DiscoverTemplateCall struct {
	Backend    string
	Project    string
	IssueType  string
	SampleSize int
}

type StubTemplateService struct {
	Template *domain.Template
	Err      error

	mu                    sync.Mutex
	DiscoverTemplateCalls []DiscoverTemplateCall
}

func (s *StubTemplateService) DiscoverTemplate(_ context.Context, backend, project, issueType string, sampleSize int) (*domain.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DiscoverTemplateCalls = append(s.DiscoverTemplateCalls, DiscoverTemplateCall{
		Backend:    backend,
		Project:    project,
		IssueType:  issueType,
		SampleSize: sampleSize,
	})
	return s.Template, s.Err
}
