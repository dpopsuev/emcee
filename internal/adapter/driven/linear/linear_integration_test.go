//go:build integration

package linear_test

import (
	"os"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/linear"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

func TestLinearContractCompliance(t *testing.T) {
	apiKey := os.Getenv("LINEAR_API_KEY")
	if apiKey == "" {
		t.Skip("LINEAR_API_KEY not set, skipping integration tests")
	}
	team := os.Getenv("LINEAR_TEAM")
	if team == "" {
		team = "HEG"
	}

	driventest.RunContractTests(t, func(t *testing.T) (driven.IssueRepository, string) {
		repo, err := linear.New(apiKey, team)
		if err != nil {
			t.Fatalf("linear.New: %v", err)
		}
		// Use a known issue key for Get/Update tests.
		// The contract test's Create will make a new issue, so we need
		// at least one existing issue. Listing will find one.
		return repo, team + "-1"
	})
}
