//nolint:dupl // stub repositories share patterns by design
package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.CommentRepository = (*StubCommentRepository)(nil)

type ListCommentsCall struct {
	Key string
}

type AddCommentCall struct {
	Key   string
	Input domain.CommentCreateInput
}

type StubCommentRepository struct {
	NameVal  string
	Comments []domain.Comment
	Comment  *domain.Comment
	Err      error

	ListCommentsErr error
	AddCommentErr   error

	mu                sync.Mutex
	ListCommentsCalls []ListCommentsCall
	AddCommentCalls   []AddCommentCall
}

func (s *StubCommentRepository) Name() string { return s.NameVal }

func (s *StubCommentRepository) ListComments(_ context.Context, key string) ([]domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListCommentsCalls = append(s.ListCommentsCalls, ListCommentsCall{Key: key})
	if s.ListCommentsErr != nil {
		return nil, s.ListCommentsErr
	}
	return s.Comments, s.Err
}

func (s *StubCommentRepository) AddComment(_ context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AddCommentCalls = append(s.AddCommentCalls, AddCommentCall{Key: key, Input: input})
	if s.AddCommentErr != nil {
		return nil, s.AddCommentErr
	}
	return s.Comment, s.Err
}
