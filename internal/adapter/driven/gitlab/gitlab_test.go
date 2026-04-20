package gitlab_test

import (
	"context"
	"os"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/gitlab"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

func TestGitLabContractCompliance(t *testing.T) {
	token := os.Getenv("GITLAB_TOKEN")
	project := os.Getenv("GITLAB_PROJECT")
	baseURL := os.Getenv("GITLAB_URL")

	if token == "" || project == "" {
		t.Skip("Skipping contract test: GITLAB_TOKEN or GITLAB_PROJECT not set")
	}

	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	// Setup function that creates a GitLab adapter and returns a known issue key
	// We need at least one issue in the project for the contract tests to work
	// Quick connectivity check
	probe, _ := gitlab.NewWithURL("gitlab", token, project, baseURL)
	if _, err := probe.List(context.Background(), domain.ListFilter{Limit: 1}); err != nil {
		t.Skipf("Skipping contract test: GitLab host unreachable: %v", err)
	}

	setup := func(t *testing.T) (driven.IssueRepository, string) {
		t.Helper()
		gl, err := gitlab.NewWithURL("gitlab", token, project, baseURL)
		if err != nil {
			t.Fatalf("Failed to create GitLab adapter: %v", err)
		}
		return gl, "#1"
	}

	driventest.RunContractTests(t, setup)
}
