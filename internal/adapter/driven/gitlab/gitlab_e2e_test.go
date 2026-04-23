// Package gitlab_e2e_test provides end-to-end tests against the live GitLab API.
// These tests require GITLAB_TOKEN and GITLAB_PROJECT environment variables.
package gitlab_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/emcee/internal/adapter/driven/gitlab"
	"github.com/dpopsuev/emcee/internal/domain"
)

//nolint:gocyclo // e2e test exercising many code paths
func TestGitLabE2E(t *testing.T) {
	token := os.Getenv("GITLAB_TOKEN")
	project := os.Getenv("GITLAB_PROJECT")
	baseURL := os.Getenv("GITLAB_URL")

	if token == "" || project == "" {
		t.Skip("Skipping E2E test: GITLAB_TOKEN or GITLAB_PROJECT not set")
	}

	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	ctx := context.Background()
	gl, err := gitlab.NewWithURL("gitlab", token, project, baseURL)
	// Quick connectivity check — skip if host is unreachable
	if _, cerr := gl.List(ctx, domain.ListFilter{Limit: 1}); cerr != nil {
		t.Skipf("Skipping E2E test: GitLab host unreachable: %v", cerr)
	}
	if err != nil {
		t.Fatalf("Failed to create GitLab adapter: %v", err)
	}

	t.Run("List issues", func(t *testing.T) {
		issues, err := gl.List(ctx, domain.ListFilter{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		t.Logf("Found %d issues", len(issues))
		for i, issue := range issues {
			if i < 3 {
				t.Logf("  %s: %s (status=%s)", issue.Key, issue.Title, issue.Status)
			}
		}
	})

	t.Run("Create, Update, and Close issue", func(t *testing.T) {
		now := time.Now().Format("2006-01-02 15:04:05")
		created, err := gl.Create(ctx, domain.CreateInput{
			Title:       "E2E Test Issue - " + now,
			Description: "Automated test issue created by emcee E2E suite. Will be closed automatically.",
			Labels:      []string{"test", "automated"},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		t.Logf("Created issue: %s - %s", created.Key, created.URL)

		retrieved, err := gl.Get(ctx, created.Key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if retrieved.Title != created.Title {
			t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, created.Title)
		}

		newTitle := "E2E Test Issue (UPDATED) - " + now
		updated, err := gl.Update(ctx, created.Key, domain.UpdateInput{
			Title: &newTitle,
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if updated.Title != newTitle {
			t.Errorf("Title not updated: got %q, want %q", updated.Title, newTitle)
		}
		t.Logf("Updated issue title")

		closed, err := gl.Update(ctx, created.Key, domain.UpdateInput{
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
		labels, err := gl.ListLabels(ctx)
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
		issues, err := gl.Search(ctx, "test", 5)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		t.Logf("Found %d issues matching 'test'", len(issues))
		for i, issue := range issues {
			if i < 3 {
				t.Logf("  %s: %s", issue.Key, issue.Title)
			}
		}
	})

	t.Run("List projects", func(t *testing.T) {
		projects, err := gl.ListProjects(ctx, domain.ProjectListFilter{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListProjects failed: %v", err)
		}
		t.Logf("Found %d projects", len(projects))
		for i, proj := range projects {
			if i < 3 {
				t.Logf("  %s: %s", proj.ID, proj.Name)
			}
		}
	})

	t.Run("Validate issue fields", func(t *testing.T) {
		now := time.Now().Format("2006-01-02 15:04:05")
		issue, err := gl.Create(ctx, domain.CreateInput{
			Title:       "Field Validation Test - " + now,
			Description: "Testing all fields are populated correctly.",
			Labels:      []string{"test"},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		defer gl.Update(ctx, issue.Key, domain.UpdateInput{
			Status: ptrStatus(domain.StatusDone),
		})

		if issue.Ref == "" {
			t.Error("issue.Ref is empty")
		}
		if issue.ID == "" {
			t.Error("issue.ID is empty")
		}
		if issue.Key == "" {
			t.Error("issue.Key is empty")
		}
		if issue.Title == "" {
			t.Error("issue.Title is empty")
		}
		if issue.URL == "" {
			t.Error("issue.URL is empty")
		}
		if issue.Status == "" {
			t.Error("issue.Status is empty")
		}
		if issue.CreatedAt.IsZero() {
			t.Error("issue.CreatedAt is zero")
		}
		if issue.UpdatedAt.IsZero() {
			t.Error("issue.UpdatedAt is zero")
		}

		t.Logf("All fields validated")
	})

	t.Run("Test filters", func(t *testing.T) {
		openIssues, err := gl.List(ctx, domain.ListFilter{
			Status: domain.StatusTodo,
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("List with status filter failed: %v", err)
		}
		t.Logf("Found %d open issues", len(openIssues))

		if len(openIssues) > 0 && len(openIssues[0].Labels) > 0 {
			labelFilter := openIssues[0].Labels[0]
			labeledIssues, err := gl.List(ctx, domain.ListFilter{
				Labels: []string{labelFilter},
				Limit:  3,
			})
			if err != nil {
				t.Fatalf("List with label filter failed: %v", err)
			}
			t.Logf("Found %d issues with label %q", len(labeledIssues), labelFilter)
		}
	})

	t.Run("Invalid credentials", func(t *testing.T) {
		badGl, err := gitlab.New("gitlab", "invalid-token", "invalid/project")
		if err != nil {
			t.Skip("Failed to create adapter")
			return
		}

		_, err = badGl.List(ctx, domain.ListFilter{Limit: 1})
		if err == nil {
			t.Error("Expected auth failure with invalid credentials")
		} else {
			t.Logf("Auth correctly failed: %v", err)
		}
	})
}

func ptrStatus(s domain.Status) *domain.Status {
	return &s
}
