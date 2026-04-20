package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/DanyPops/emcee/internal/adapter/driven/sqlite"
	"github.com/DanyPops/emcee/internal/domain"
)

func newTestLedger(t *testing.T) *sqlite.Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	l, err := sqlite.NewLedger(path)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestPutAndGet(t *testing.T) {
	l := newTestLedger(t)
	ctx := context.Background()

	rec := domain.ArtifactRecord{
		Ref:        "jira:PROJ-1",
		Backend:    "jira",
		Type:       "issue",
		Title:      "Fix login bug",
		Status:     "open",
		Labels:     []string{"bug", "urgent"},
		Components: []string{"auth"},
		Text:       "Login fails when password contains special chars",
		SeenAt:     time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := l.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := l.Get(ctx, "jira:PROJ-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", got.Title, "Fix login bug")
	}
	if len(got.Labels) != 2 || got.Labels[0] != "bug" {
		t.Errorf("Labels = %v, want [bug urgent]", got.Labels)
	}
}

func TestGetNotFound(t *testing.T) {
	l := newTestLedger(t)
	_, err := l.Get(context.Background(), "nope:X")
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
}

func TestPutUpsert(t *testing.T) {
	l := newTestLedger(t)
	ctx := context.Background()

	rec := domain.ArtifactRecord{Ref: "jira:UPS-1", Backend: "jira", Title: "v1", SeenAt: time.Now(), UpdatedAt: time.Now()}
	_ = l.Put(ctx, rec)

	rec.Title = "v2"
	_ = l.Put(ctx, rec)

	got, _ := l.Get(ctx, "jira:UPS-1")
	if got.Title != "v2" {
		t.Errorf("Title after upsert = %q, want %q", got.Title, "v2")
	}
}

func TestListWithFilter(t *testing.T) {
	l := newTestLedger(t)
	ctx := context.Background()

	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:A", Backend: "jira", Type: "issue", Status: "open", SeenAt: time.Now(), UpdatedAt: time.Now()})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "github:B", Backend: "github", Type: "pr", Status: "merged", SeenAt: time.Now(), UpdatedAt: time.Now()})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:C", Backend: "jira", Type: "issue", Status: "done", SeenAt: time.Now(), UpdatedAt: time.Now()})

	recs, err := l.List(ctx, domain.LedgerFilter{Backend: "jira"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("List(backend=jira) got %d, want 2", len(recs))
	}

	recs, err = l.List(ctx, domain.LedgerFilter{Status: "open"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("List(status=open) got %d, want 1", len(recs))
	}
}

func TestSearch(t *testing.T) {
	l := newTestLedger(t)
	ctx := context.Background()

	_ = l.Put(ctx, domain.ArtifactRecord{
		Ref: "jira:SRCH-1", Backend: "jira", Title: "Authentication failure on login",
		Text: "Users report 401 errors when using SSO", SeenAt: time.Now(), UpdatedAt: time.Now(),
	})
	_ = l.Put(ctx, domain.ArtifactRecord{
		Ref: "jira:SRCH-2", Backend: "jira", Title: "Database migration timeout",
		Text: "Migration takes too long on large datasets", SeenAt: time.Now(), UpdatedAt: time.Now(),
	})

	recs, err := l.Search(ctx, "authentication", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("Search(authentication) got %d, want 1", len(recs))
	}
	if recs[0].Ref != "jira:SRCH-1" {
		t.Errorf("Search result ref = %q, want jira:SRCH-1", recs[0].Ref)
	}
}

func TestStats(t *testing.T) {
	l := newTestLedger(t)
	ctx := context.Background()

	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:S1", Backend: "jira", SeenAt: time.Now(), UpdatedAt: time.Now()})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "jira:S2", Backend: "jira", SeenAt: time.Now(), UpdatedAt: time.Now()})
	_ = l.Put(ctx, domain.ArtifactRecord{Ref: "github:S3", Backend: "github", SeenAt: time.Now(), UpdatedAt: time.Now()})

	stats, err := l.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.ByBackend["jira"] != 2 {
		t.Errorf("ByBackend[jira] = %d, want 2", stats.ByBackend["jira"])
	}
}
