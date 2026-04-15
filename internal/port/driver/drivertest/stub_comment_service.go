//nolint:dupl // stub services share patterns by design
package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.CommentService = (*StubCommentService)(nil)

type CommentListCall struct {
	Ref string
}

type CommentAddCall struct {
	Ref   string
	Input domain.CommentCreateInput
}

type StubCommentService struct {
	Comments []domain.Comment
	Comment  *domain.Comment
	Err      error

	mu                sync.Mutex
	ListCommentsCalls []CommentListCall
	AddCommentCalls   []CommentAddCall
}

func (s *StubCommentService) ListComments(_ context.Context, ref string) ([]domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListCommentsCalls = append(s.ListCommentsCalls, CommentListCall{Ref: ref})
	return s.Comments, s.Err
}

func (s *StubCommentService) AddComment(_ context.Context, ref string, input domain.CommentCreateInput) (*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AddCommentCalls = append(s.AddCommentCalls, CommentAddCall{Ref: ref, Input: input})
	return s.Comment, s.Err
}
