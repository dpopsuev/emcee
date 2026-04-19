package cache_test

import (
	"context"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/cache"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

// asIssueRepo casts to the interface that app.NewService uses for type assertions.
func asIssueRepo(r *cache.Repository) driven.IssueRepository { return r }

// multiRepo implements IssueRepository + LaunchRepository + FieldRepository + JQLRepository
// to simulate Report Portal or Jira backends that support multiple interfaces.
type multiRepo struct {
	driventest.StubIssueRepository
	driventest.StubLaunchRepository
	driventest.StubFieldRepository
	driventest.StubJQLRepository
	driventest.StubBuildRepository
}

func (m *multiRepo) Name() string { return m.StubIssueRepository.NameVal }

// --- BUG-7: Cache swallows interfaces ---

func TestCachePassthroughLaunchRepository(t *testing.T) {
	inner := &multiRepo{}
	inner.StubIssueRepository.NameVal = "reportportal"
	inner.StubLaunchRepository.Launches = []domain.Launch{{ID: "1", Name: "test-launch"}}

	wrapped := asIssueRepo(cache.New(inner))

	// The cache wrapper should pass through LaunchRepository
	lr, ok := wrapped.(driven.LaunchRepository)
	if !ok {
		t.Fatal("cache.Repository does not implement LaunchRepository — inner capabilities swallowed by decorator")
	}

	launches, err := lr.ListLaunches(context.Background(), domain.LaunchFilter{})
	if err != nil {
		t.Fatalf("ListLaunches: %v", err)
	}
	if len(launches) != 1 || launches[0].Name != "test-launch" {
		t.Errorf("got %v, want [{ID:1 Name:test-launch}]", launches)
	}
}

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

func TestCachePassthroughBuildRepository(t *testing.T) {
	inner := &multiRepo{}
	inner.StubIssueRepository.NameVal = "jenkins"
	inner.StubBuildRepository.Jobs = []domain.Job{{Name: "my-pipeline", Buildable: true}}

	wrapped := asIssueRepo(cache.New(inner))

	br, ok := wrapped.(driven.BuildRepository)
	if !ok {
		t.Fatal("cache.Repository does not implement BuildRepository — inner capabilities swallowed by decorator")
	}

	jobs, err := br.ListJobs(context.Background(), domain.JobFilter{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "my-pipeline" {
		t.Errorf("got %v, want [{Name:my-pipeline}]", jobs)
	}
}

func TestCacheNoBuildWhenInnerLacks(t *testing.T) {
	inner := driventest.NewStubIssueRepository("linear")

	wrapped := asIssueRepo(cache.New(inner))

	br, ok := wrapped.(driven.BuildRepository)
	if !ok {
		t.Fatal("cache should always implement BuildRepository (returns error if inner doesn't)")
	}
	_, err := br.ListJobs(context.Background(), domain.JobFilter{})
	if err == nil {
		t.Fatal("expected error when inner doesn't support builds")
	}
}

func TestCacheNoLaunchWhenInnerLacks(t *testing.T) {
	// Plain IssueRepository without LaunchRepository — cache has the methods
	// but they return ErrNotSupported at runtime
	inner := driventest.NewStubIssueRepository("linear")

	wrapped := asIssueRepo(cache.New(inner))

	lr, ok := wrapped.(driven.LaunchRepository)
	if !ok {
		t.Fatal("cache should always implement LaunchRepository (returns error if inner doesn't)")
	}
	_, err := lr.ListLaunches(context.Background(), domain.LaunchFilter{})
	if err == nil {
		t.Fatal("expected error when inner doesn't support launches")
	}
}
