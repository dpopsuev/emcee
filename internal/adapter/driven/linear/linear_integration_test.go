//go:build integration

package linear_test

import (
	"context"
	"os"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/linear"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

func newTestRepo(t *testing.T) *linear.Repository {
	t.Helper()
	apiKey := os.Getenv("LINEAR_API_KEY")
	if apiKey == "" {
		t.Skip("LINEAR_API_KEY not set, skipping integration tests")
	}
	team := os.Getenv("LINEAR_TEAM")
	if team == "" {
		team = "HEG"
	}
	repo, err := linear.New("linear", apiKey, team)
	if err != nil {
		t.Fatalf("linear.New: %v", err)
	}
	return repo
}

func TestLinearContractCompliance(t *testing.T) {
	repo := newTestRepo(t)
	team := os.Getenv("LINEAR_TEAM")
	if team == "" {
		team = "HEG"
	}
	driventest.RunContractTests(t, func(t *testing.T) (driven.IssueRepository, string) {
		return repo, team + "-1"
	})
}

func TestLinearDocuments(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("ListDocuments", func(t *testing.T) {
		docs, err := repo.ListDocuments(context.Background(), domain.DocumentListFilter{Limit: 5})
		if err != nil {
			t.Fatalf("ListDocuments: %v", err)
		}
		t.Logf("found %d documents", len(docs))
	})

	t.Run("CreateDocument", func(t *testing.T) {
		doc, err := repo.CreateDocument(context.Background(), domain.DocumentCreateInput{
			Title:   "Integration test document",
			Content: "Created by emcee integration test",
		})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		if doc.Title != "Integration test document" {
			t.Errorf("title = %q", doc.Title)
		}
		t.Logf("created document: %s", doc.ID)
	})
}

func TestLinearProjects(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("ListProjects", func(t *testing.T) {
		projs, err := repo.ListProjects(context.Background(), domain.ProjectListFilter{Limit: 5})
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		t.Logf("found %d projects", len(projs))
	})
}

func TestLinearInitiatives(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("ListInitiatives", func(t *testing.T) {
		inits, err := repo.ListInitiatives(context.Background(), domain.InitiativeListFilter{Limit: 5})
		if err != nil {
			t.Fatalf("ListInitiatives: %v", err)
		}
		t.Logf("found %d initiatives", len(inits))
	})
}

func TestLinearLabels(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("ListLabels", func(t *testing.T) {
		labels, err := repo.ListLabels(context.Background())
		if err != nil {
			t.Fatalf("ListLabels: %v", err)
		}
		t.Logf("found %d labels", len(labels))
	})
}

func TestLinearResolveUser(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("ResolveUser", func(t *testing.T) {
		_, err := repo.ResolveUser(context.Background(), "nonexistent-user-xyz")
		if err == nil {
			t.Error("expected error for nonexistent user")
		}
	})
}
