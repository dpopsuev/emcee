package drivertest

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.PRReviewService = (*StubPRReviewService)(nil)

type StubPRReviewService struct {
	Reviews    []domain.PRReview
	PRComments []domain.PRComment
	Err        error
}

func (s *StubPRReviewService) ListPRReviews(_ context.Context, _ string, _ int) ([]domain.PRReview, error) {
	return s.Reviews, s.Err
}

func (s *StubPRReviewService) ListPRComments(_ context.Context, _ string, _ int) ([]domain.PRComment, error) {
	return s.PRComments, s.Err
}
