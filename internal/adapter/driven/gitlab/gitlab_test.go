package gitlab_test

import (
	"os"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/gitlab"
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
	setup := func(t *testing.T) (driven.IssueRepository, string) {
		t.Helper()
		gl, err := gitlab.NewWithURL("gitlab", token, project, baseURL)
		if err != nil {
			t.Fatalf("Failed to create GitLab adapter: %v", err)
		}
		// Return the adapter and use "#1" as a known issue key (most projects have #1)
		// If your project doesn't have issue #1, change this to an issue that exists
		return gl, "#1"
	}

	driventest.RunContractTests(t, setup)
}
