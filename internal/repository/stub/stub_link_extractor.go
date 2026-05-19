package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	repository "github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.LinkExtractor = (*StubLinkExtractor)(nil)

type ExtractCall struct {
	Text string
}

type StubLinkExtractor struct {
	Refs []domain.CrossRef
	Err  error

	mu           sync.Mutex
	ExtractCalls []ExtractCall
}

func (s *StubLinkExtractor) Extract(_ context.Context, text string) ([]domain.CrossRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExtractCalls = append(s.ExtractCalls, ExtractCall{Text: text})
	return s.Refs, s.Err
}
