// Package github_e2e_test provides end-to-end tests against the live GitHub API.
// These tests require GITHUB_TOKEN, GITHUB_OWNER, and GITHUB_REPO environment variables.
// They will be skipped if not present.
package github_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DanyPops/emcee/internal/adapter/driven/github"
	"github.com/DanyPops/emcee/internal/domain"
)

//nolint:gocyclo // e2e test exercising many code paths
func TestGitHubE2E(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("GITHUB_OWNER")
	repo := os.Getenv("GITHUB_REPO")

	if token == "" || owner == "" || repo == "" {
		t.Skip("Skipping E2E test: GITHUB_TOKEN, GITHUB_OWNER, or GITHUB_REPO not set")
	}

	ctx := context.Background()
	gh, err := github.New("github", token, owner, repo)
	if err != nil {
		t.Fatalf("Failed to create GitHub adapter: %v", err)
	}

	t.Run("List issues", func(t *testing.T) {
		issues, err := gh.List(ctx, domain.ListFilter{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		t.Logf("Found %d issues", len(issues))
		for _, issue := range issues {
			t.Logf("  %s: %s (status=%s)", issue.Key, issue.Title, issue.Status)
		}
	})

	t.Run("Create, Update, and Close issue (dogfood)", func(t *testing.T) {
		// Create a test issue
		now := time.Now().Format("2006-01-02 15:04:05")
		created, err := gh.Create(ctx, domain.CreateInput{
			Title:       "E2E Test Issue - " + now,
			Description: "This is an automated test issue created by emcee's E2E test suite. It will be closed automatically.",
			Labels:      []string{"test", "automated"},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		t.Logf("Created issue: %s - %s", created.Key, created.URL)

		// Verify we can get the issue
		retrieved, err := gh.Get(ctx, created.Key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if retrieved.Title != created.Title {
			t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, created.Title)
		}

		// Update the issue
		newTitle := "E2E Test Issue (UPDATED) - " + now
		updated, err := gh.Update(ctx, created.Key, domain.UpdateInput{
			Title: &newTitle,
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if updated.Title != newTitle {
			t.Errorf("Title not updated: got %q, want %q", updated.Title, newTitle)
		}
		t.Logf("Updated issue title")

		// Close the issue
		closed, err := gh.Update(ctx, created.Key, domain.UpdateInput{
			Status: ptrStatus(domain.StatusDone),
		})
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
		if closed.Status != domain.StatusDone {
			t.Errorf("Status not updated: got %v, want %v", closed.Status, domain.StatusDone)
		}
		t.Logf("Closed issue: %s", closed.URL)
	})

	t.Run("List labels", func(t *testing.T) {
		labels, err := gh.ListLabels(ctx)
		if err != nil {
			t.Fatalf("ListLabels failed: %v", err)
		}
		t.Logf("Found %d labels", len(labels))
		for i, label := range labels {
			if i < 5 {
				t.Logf("  %s", label.Name)
			}
		}
	})

	t.Run("Search issues", func(t *testing.T) {
		issues, err := gh.Search(ctx, "test", 5)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		t.Logf("Found %d issues matching 'test'", len(issues))
		for _, issue := range issues {
			t.Logf("  %s: %s", issue.Key, issue.Title)
		}
	})

	t.Run("List projects", func(t *testing.T) {
		projects, err := gh.ListProjects(ctx, domain.ProjectListFilter{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListProjects failed: %v", err)
		}
		t.Logf("Found %d projects", len(projects))
		for _, proj := range projects {
			t.Logf("  %s: %s", proj.ID, proj.Name)
		}
	})
}

func ptrStatus(s domain.Status) *domain.Status {
	return &s
}
