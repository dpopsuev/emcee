package github_test

import (
	"os"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/github"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

func TestGitHubContractCompliance(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("GITHUB_OWNER")
	repo := os.Getenv("GITHUB_REPO")

	if token == "" || owner == "" || repo == "" {
		t.Skip("Skipping contract test: GITHUB_TOKEN, GITHUB_OWNER, or GITHUB_REPO not set")
	}

	// Setup function that creates a GitHub adapter and returns a known issue key
	// We need at least one issue in the repo for the contract tests to work
	setup := func(t *testing.T) (driven.IssueRepository, string) {
		t.Helper()
		gh, err := github.New("github", token, owner, repo)
		if err != nil {
			t.Fatalf("Failed to create GitHub adapter: %v", err)
		}
		// Return the adapter and use "#1" as a known issue key (most repos have #1)
		// If your repo doesn't have issue #1, change this to an issue that exists
		return gh, "#1"
	}

	driventest.RunContractTests(t, setup)
}
