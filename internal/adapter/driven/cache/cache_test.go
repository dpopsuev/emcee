package cache_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/emcee/internal/adapter/driven/cache"
	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
	"github.com/dpopsuev/emcee/internal/port/driven/driventest"
)

// asIssueRepo casts to the interface that app.NewService uses for type assertions.
func asIssueRepo(r *cache.Repository) driven.IssueRepository { return r }

// multiRepo implements IssueRepository + FieldRepository + JQLRepository
// to simulate Jira backends that support multiple interfaces.
type multiRepo struct {
	driventest.StubIssueRepository
	driventest.StubFieldRepository
	driventest.StubJQLRepository
}

func (m *multiRepo) Name() string { return m.StubIssueRepository.NameVal }

// --- BUG-7: Cache swallows interfaces ---

func TestCachePassthroughFieldRepository(t *testing.T) {
	inner := &multiRepo{}
	inner.StubIssueRepository.NameVal = "jira"
	inner.StubFieldRepository.Fields = []domain.Field{{ID: "customfield_123", Name: "Sprint", Custom: true}}

	wrapped := asIssueRepo(cache.New(inner))

	fr, ok := wrapped.(driven.FieldRepository)
	if !ok {
		t.Fatal("cache.Repository does not implement FieldRepository — inner capabilities swallowed by decorator")
	}

	fields, err := fr.ListFields(context.Background())
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "Sprint" {
		t.Errorf("got %v, want [{ID:customfield_123 Name:Sprint}]", fields)
	}
}

func TestCachePassthroughJQLRepository(t *testing.T) {
	inner := &multiRepo{}
	inner.StubIssueRepository.NameVal = "jira"
	inner.StubJQLRepository.Issues = []domain.Issue{{Key: "PROJ-1", Title: "found"}}

	wrapped := asIssueRepo(cache.New(inner))

	jr, ok := wrapped.(driven.JQLRepository)
	if !ok {
		t.Fatal("cache.Repository does not implement JQLRepository — inner capabilities swallowed by decorator")
	}

	issues, err := jr.SearchJQL(context.Background(), "project = PROJ", 10)
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
	if len(issues) != 1 || issues[0].Key != "PROJ-1" {
		t.Errorf("got %v, want [{Key:PROJ-1}]", issues)
	}
}

func TestCachePassthroughCommentRepository(t *testing.T) {
	// CommentRepository is already handled — verify it still works
	inner := &multiRepo{}
	inner.StubIssueRepository.NameVal = "test"

	wrapped := asIssueRepo(cache.New(inner))

	_, ok := wrapped.(driven.CommentRepository)
	if !ok {
		t.Fatal("cache.Repository does not implement CommentRepository")
	}
}

