package gitlab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/gitlab"
	"github.com/DanyPops/emcee/internal/domain"
)

func newReadOnlyRepo(t *testing.T) *gitlab.Repository {
	t.Helper()
	repo, err := gitlab.NewWithURL("gitlab", "", "some/project", "https://gitlab.com")
	if err != nil {
		t.Fatalf("NewWithURL: %v", err)
	}
	return repo
}

func TestReadOnlyCreate(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.Create(context.Background(), domain.CreateInput{Title: "test"})
	if !errors.Is(err, gitlab.ErrAuthRequired) {
		t.Errorf("Create err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyUpdate(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.Update(context.Background(), "1", domain.UpdateInput{})
	if !errors.Is(err, gitlab.ErrAuthRequired) {
		t.Errorf("Update err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyAddComment(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.AddComment(context.Background(), "1", domain.CommentCreateInput{Body: "test"})
	if !errors.Is(err, gitlab.ErrAuthRequired) {
		t.Errorf("AddComment err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyCreateLabel(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.CreateLabel(context.Background(), domain.LabelCreateInput{Name: "test"})
	if !errors.Is(err, gitlab.ErrAuthRequired) {
		t.Errorf("CreateLabel err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyRetryPipeline(t *testing.T) {
	r := newReadOnlyRepo(t)
	err := r.RetryPipeline(context.Background(), 1)
	if !errors.Is(err, gitlab.ErrAuthRequired) {
		t.Errorf("RetryPipeline err = %v, want ErrAuthRequired", err)
	}
}
