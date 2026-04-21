package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/github"
	"github.com/DanyPops/emcee/internal/domain"
)

func newReadOnlyRepo(t *testing.T) *github.Repository {
	t.Helper()
	repo, err := github.NewWithURL("github", "", "DanyPops", "emcee", "https://api.github.com")
	if err != nil {
		t.Fatalf("NewWithURL: %v", err)
	}
	return repo
}

func TestReadOnlyCreate(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.Create(context.Background(), domain.CreateInput{Title: "test"})
	if !errors.Is(err, github.ErrAuthRequired) {
		t.Errorf("Create err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyUpdate(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.Update(context.Background(), "1", domain.UpdateInput{})
	if !errors.Is(err, github.ErrAuthRequired) {
		t.Errorf("Update err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyAddComment(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.AddComment(context.Background(), "1", domain.CommentCreateInput{Body: "test"})
	if !errors.Is(err, github.ErrAuthRequired) {
		t.Errorf("AddComment err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyCreateLabel(t *testing.T) {
	r := newReadOnlyRepo(t)
	_, err := r.CreateLabel(context.Background(), domain.LabelCreateInput{Name: "test"})
	if !errors.Is(err, github.ErrAuthRequired) {
		t.Errorf("CreateLabel err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyRerunFailed(t *testing.T) {
	r := newReadOnlyRepo(t)
	err := r.RerunFailedJobs(context.Background(), 1)
	if !errors.Is(err, github.ErrAuthRequired) {
		t.Errorf("RerunFailedJobs err = %v, want ErrAuthRequired", err)
	}
}

func TestReadOnlyInitNoOwner(t *testing.T) {
	_, err := github.NewWithURL("github", "", "", "", "https://api.github.com")
	if !errors.Is(err, github.ErrOwnerRequired) {
		t.Errorf("err = %v, want ErrOwnerRequired", err)
	}
}

func TestReadOnlyInitNoRepo(t *testing.T) {
	r, err := github.NewWithURL("github", "", "DanyPops", "", "https://api.github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil repo")
	}
}
