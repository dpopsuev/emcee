// Package driventest provides a shared contract test suite that any
// IssueRepository implementation must pass.
package driventest

import (
	"context"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// RunContractTests exercises the IssueRepository contract against any implementation.
// The setup function should return a repository pre-loaded with at least one issue,
// along with the key of that issue for Get/Update tests.
func RunContractTests(t *testing.T, setup func(t *testing.T) (driven.IssueRepository, string)) {
	t.Helper()

	t.Run("Name", func(t *testing.T) {
		repo, _ := setup(t)
		if name := repo.Name(); name == "" {
			t.Error("Name() returned empty string")
		}
	})

	t.Run("List", func(t *testing.T) {
		repo, _ := setup(t)
		issues, err := repo.List(context.Background(), domain.ListFilter{})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(issues) == 0 {
			t.Error("List returned no issues, expected at least one")
		}
		for _, issue := range issues {
			assertIssueFields(t, issue)
		}
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		repo, _ := setup(t)
		issues, err := repo.List(context.Background(), domain.ListFilter{Limit: 1})
		if err != nil {
			t.Fatalf("List(limit=1): %v", err)
		}
		if len(issues) > 1 {
			t.Errorf("List(limit=1) returned %d issues, want at most 1", len(issues))
		}
	})

	t.Run("Get", func(t *testing.T) {
		repo, key := setup(t)
		issue, err := repo.Get(context.Background(), key)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		if issue == nil {
			t.Fatal("Get returned nil issue")
		}
		assertIssueFields(t, *issue)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		repo, _ := setup(t)
		_, err := repo.Get(context.Background(), "NONEXISTENT-99999")
		if err == nil {
			t.Error("Get(nonexistent) should return error")
		}
	})

	t.Run("Create", func(t *testing.T) {
		repo, _ := setup(t)
		input := domain.CreateInput{Title: "Contract test issue"}
		issue, err := repo.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if issue == nil {
			t.Fatal("Create returned nil issue")
		}
		if issue.Title != "Contract test issue" {
			t.Errorf("title = %q, want %q", issue.Title, "Contract test issue")
		}
		assertIssueFields(t, *issue)
	})

	t.Run("Update", func(t *testing.T) {
		repo, key := setup(t)
		newTitle := "Updated by contract test"
		issue, err := repo.Update(context.Background(), key, domain.UpdateInput{Title: &newTitle})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if issue == nil {
			t.Fatal("Update returned nil issue")
		}
		if issue.Title != newTitle {
			t.Errorf("title = %q, want %q", issue.Title, newTitle)
		}
	})

	t.Run("Search", func(t *testing.T) {
		repo, _ := setup(t)
		results, err := repo.Search(context.Background(), "test", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		_ = results
	})
}

func assertIssueFields(t *testing.T, issue domain.Issue) {
	t.Helper()
	if issue.Ref == "" {
		t.Error("issue.Ref is empty")
	}
	if issue.Key == "" {
		t.Error("issue.Key is empty")
	}
	if issue.Title == "" {
		t.Error("issue.Title is empty")
	}
}
