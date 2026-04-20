package triage

import (
	"context"
	"testing"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
)

func TestInMemoryLedger_PutAndGet(t *testing.T) {
	l := NewInMemoryLedger()
	ctx := context.Background()

	rec := domain.ArtifactRecord{
		Ref:     "jira:BUG-1",
		Backend: "jira",
		Type:    "issue",
		Title:   "Bug One",
		SeenAt:  time.Now(),
	}
	if err := l.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := l.Get(ctx, "jira:BUG-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Bug One" {
		t.Errorf("title = %q, want %q", got.Title, "Bug One")
	}
	if got.Backend != "jira" {
		t.Errorf("backend = %q, want %q", got.Backend, "jira")
	}
}

func TestInMemoryLedger_GetNotFound(t *testing.T) {
	l := NewInMemoryLedger()
	ctx := context.Background()

	_, err := l.Get(ctx, "nonexistent:KEY-1")
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
}

func TestInMemoryLedger_Upsert(t *testing.T) {
	l := NewInMemoryLedger()
	ctx := context.Background()

	rec := domain.ArtifactRecord{
		Ref:     "github:org/repo#42",
		Backend: "github",
		Type:    "issue",
		Title:   "Original",
		SeenAt:  time.Now(),
	}
	_ = l.Put(ctx, rec)

	rec.Title = "Updated"
	_ = l.Put(ctx, rec)

	got, _ := l.Get(ctx, "github:org/repo#42")
	if got.Title != "Updated" {
		t.Errorf("title = %q, want %q after upsert", got.Title, "Updated")
	}

	stats, _ := l.Stats(ctx)
	if stats.Total != 1 {
		t.Errorf("total = %d, want 1 (upsert should not duplicate)", stats.Total)
	}
}

func TestInMemoryLedger_ListFilters(t *testing.T) {
	l := NewInMemoryLedger()
	ctx := context.Background()

	now := time.Now()
	records := []domain.ArtifactRecord{
		{Ref: "jira:BUG-1", Backend: "jira", Type: "issue", Status: "open", Components: []string{"ui"}, SeenAt: now},
		{Ref: "jira:BUG-2", Backend: "jira", Type: "issue", Status: "closed", Components: []string{"api"}, SeenAt: now},
		{Ref: "github:org/repo#1", Backend: "github", Type: "issue", Status: "open", Components: []string{"ui"}, SeenAt: now},
		{Ref: "jenkins:job#99", Backend: "jenkins", Type: "build", Status: "success", SeenAt: now},
	}
	for _, r := range records {
		_ = l.Put(ctx, r)
	}

	// Filter by backend
	got, _ := l.List(ctx, domain.LedgerFilter{Backend: "jira"})
	if len(got) != 2 {
		t.Errorf("backend=jira: got %d, want 2", len(got))
	}

	// Filter by type
	got, _ = l.List(ctx, domain.LedgerFilter{Type: "build"})
	if len(got) != 1 {
		t.Errorf("type=build: got %d, want 1", len(got))
	}

	// Filter by status
	got, _ = l.List(ctx, domain.LedgerFilter{Status: "open"})
	if len(got) != 2 {
		t.Errorf("status=open: got %d, want 2", len(got))
	}

	// Filter by component
	got, _ = l.List(ctx, domain.LedgerFilter{Component: "ui"})
	if len(got) != 2 {
		t.Errorf("component=ui: got %d, want 2", len(got))
	}

	// Limit
	got, _ = l.List(ctx, domain.LedgerFilter{Limit: 1})
	if len(got) != 1 {
		t.Errorf("limit=1: got %d, want 1", len(got))
	}

	// No filter
	got, _ = l.List(ctx, domain.LedgerFilter{})
	if len(got) != 4 {
		t.Errorf("no filter: got %d, want 4", len(got))
	}
}

func TestInMemoryLedger_StatsByBackend(t *testing.T) {
	l := NewInMemoryLedger()
	ctx := context.Background()

	now := time.Now()
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:A", Backend: "jira", SeenAt: now})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:B", Backend: "jira", SeenAt: now})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "github:C", Backend: "github", SeenAt: now})

	stats, err := l.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("total = %d, want 3", stats.Total)
	}
	if stats.ByBackend["jira"] != 2 {
		t.Errorf("by_backend[jira] = %d, want 2", stats.ByBackend["jira"])
	}
	if stats.ByBackend["github"] != 1 {
		t.Errorf("by_backend[github] = %d, want 1", stats.ByBackend["github"])
	}
}
