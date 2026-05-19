package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

var _ repository.PRReviewRepository = (*Repository)(nil)

// ListPRReviews lists reviews on a pull request.
func (r *Repository) ListPRReviews(ctx context.Context, prNumber int) ([]domain.PRReview, error) {
	infra.LogOp(ctx, BackendName, "list_pr_reviews")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/pulls/%d/reviews", rp, prNumber)
	var raw []struct {
		ID   int `json:"id"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State string `json:"state"`
		Body  string `json:"body"`
	}
	if err := r.api(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	reviews := make([]domain.PRReview, len(raw))
	for i := range raw {
		reviews[i] = domain.PRReview{
			ID:     strconv.Itoa(raw[i].ID),
			Author: raw[i].User.Login,
			State:  raw[i].State,
			Body:   raw[i].Body,
		}
	}
	infra.LogOpDone(ctx, BackendName, "list_pr_reviews", slog.Duration(infra.LogKeyElapsed, time.Since(start)))
	return reviews, nil
}

// ListPRComments lists review comments on a pull request's diff.
func (r *Repository) ListPRComments(ctx context.Context, prNumber int) ([]domain.PRComment, error) {
	infra.LogOp(ctx, BackendName, "list_pr_comments")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/pulls/%d/comments?per_page=%d", rp, prNumber, defaultLimit)
	var raw []struct {
		ID   int `json:"id"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body     string `json:"body"`
		Path     string `json:"path"`
		Line     int    `json:"line"`
		CommitID string `json:"commit_id"`
	}
	if err := r.api(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	comments := make([]domain.PRComment, len(raw))
	for i := range raw {
		comments[i] = domain.PRComment{
			ID:       strconv.Itoa(raw[i].ID),
			Author:   raw[i].User.Login,
			Body:     raw[i].Body,
			Path:     raw[i].Path,
			Line:     raw[i].Line,
			CommitID: raw[i].CommitID,
		}
	}
	infra.LogOpDone(ctx, BackendName, "list_pr_comments", slog.Duration(infra.LogKeyElapsed, time.Since(start)))
	return comments, nil
}
