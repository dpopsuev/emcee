package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.IssueLinkRepository = (*StubIssueLinkRepository)(nil)

type StubIssueLinkRepository struct {
	NameVal string
	Err     error

	mu                   sync.Mutex
	CreateIssueLinkCalls []domain.IssueLinkInput
}

func (s *StubIssueLinkRepository) Name() string { return s.NameVal }

func (s *StubIssueLinkRepository) CreateIssueLink(_ context.Context, input domain.IssueLinkInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CreateIssueLinkCalls = append(s.CreateIssueLinkCalls, input)
	return s.Err
}
