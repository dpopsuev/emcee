package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/emcee/internal/application"
	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/repository/stub"
)

func newViewTestService(t *testing.T) (*application.Service, *stub.StubIssueRepository) {
	t.Helper()
	stub := stub.NewStubIssueRepository("test")
	stub.Issue = &domain.Issue{
		Key:         "PROJ-1",
		Title:       "Fix bug",
		Description: "It's broken",
		Status:      domain.StatusTodo,
		Priority:    domain.PriorityHigh,
		Assignee:    "alice",
		URL:         "https://example.com/PROJ-1",
		UpdatedAt:   time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
	}
	svc := application.NewService(stub)
	return svc, stub
}

func TestViewPull(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	vr, err := svc.ViewPull(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("ViewPull: %v", err)
	}
	if vr.Ref != "test:PROJ-1" {
		t.Errorf("ref = %q, want test:PROJ-1", vr.Ref)
	}
	if vr.Fields["title"] != "Fix bug" {
		t.Errorf("title = %q, want 'Fix bug'", vr.Fields["title"])
	}
	if vr.Fields["status"] != "todo" {
		t.Errorf("status = %q, want 'todo'", vr.Fields["status"])
	}
}

func TestViewPull_IdentityMap(t *testing.T) {
	svc, stub := newViewTestService(t)
	ctx := context.Background()

	_, err := svc.ViewPull(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("first pull: %v", err)
	}

	stub.Issue.Title = "Updated title"
	_, err = svc.ViewPull(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("second pull: %v", err)
	}

	vr, _ := svc.ViewGet("test:PROJ-1")
	if vr.Fields["title"] != "Updated title" {
		t.Errorf("expected re-pull to update, got %q", vr.Fields["title"])
	}
}

func TestViewGet_NotFound(t *testing.T) {
	svc, _ := newViewTestService(t)

	_, err := svc.ViewGet("test:NONEXIST")
	if err == nil {
		t.Error("expected error for non-pulled ref")
	}
}

func TestViewMutate(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, err := svc.ViewPull(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}

	if err := svc.ViewMutate("test:PROJ-1", "status", "done"); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	vr, _ := svc.ViewGet("test:PROJ-1")
	if vr.Fields["status"] != "done" {
		t.Errorf("status = %q, want 'done'", vr.Fields["status"])
	}
}

func TestViewMutate_Noop(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")

	if err := svc.ViewMutate("test:PROJ-1", "status", "todo"); err != nil {
		t.Fatalf("mutate same value: %v", err)
	}

	dirty := svc.ViewDirty()
	if len(dirty) != 0 {
		t.Errorf("expected no dirty sets for noop mutation, got %d", len(dirty))
	}
}

func TestViewMutate_NotPulled(t *testing.T) {
	svc, _ := newViewTestService(t)

	err := svc.ViewMutate("test:NONEXIST", "status", "done")
	if err == nil {
		t.Error("expected error for non-pulled ref")
	}
}

func TestViewDiff(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	_ = svc.ViewMutate("test:PROJ-1", "status", "done")
	_ = svc.ViewMutate("test:PROJ-1", "assignee", "bob")

	diff, err := svc.ViewDiff("test:PROJ-1")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff.Ref != "test:PROJ-1" {
		t.Errorf("ref = %q, want test:PROJ-1", diff.Ref)
	}
	if len(diff.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(diff.Changes))
	}
}

func TestViewDiff_Clean(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")

	diff, err := svc.ViewDiff("test:PROJ-1")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Changes) != 0 {
		t.Errorf("expected no changes for clean record, got %d", len(diff.Changes))
	}
}

func TestViewPush(t *testing.T) {
	svc, stub := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	_ = svc.ViewMutate("test:PROJ-1", "status", "done")
	// Stub stays at StatusTodo (pulled state) — remote hasn't changed the same field,
	// so conflict check passes. Update then returns the same stub as post-update.

	issue, err := svc.ViewPush(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue from push")
	}

	if len(stub.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(stub.UpdateCalls))
	}
	update := stub.UpdateCalls[0]
	if update.Input.Status == nil || *update.Input.Status != domain.StatusDone {
		t.Error("expected status=done in update")
	}
	if update.Input.Title != nil {
		t.Error("expected title to be nil (not dirty)")
	}

	dirty := svc.ViewDirty()
	if len(dirty) != 0 {
		t.Errorf("expected clean after push, got %d dirty", len(dirty))
	}
}

func TestViewPush_NothingDirty(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")

	issue, err := svc.ViewPush(ctx, "test:PROJ-1")
	if err != nil {
		t.Fatalf("push clean: %v", err)
	}
	if issue != nil {
		t.Error("expected nil issue for clean push")
	}
}

func TestViewList(t *testing.T) {
	svc, stub := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	stub.Issue = &domain.Issue{
		Key:       "PROJ-2",
		Title:     "Another bug",
		Status:    domain.StatusBacklog,
		UpdatedAt: time.Now(),
	}
	_, _ = svc.ViewPull(ctx, "test:PROJ-2")

	list := svc.ViewList()
	if len(list) != 2 {
		t.Errorf("expected 2 records, got %d", len(list))
	}
}

func TestViewDrop(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	svc.ViewDrop("test:PROJ-1")

	_, err := svc.ViewGet("test:PROJ-1")
	if err == nil {
		t.Error("expected error after drop")
	}
}

func TestViewPush_ConflictDetection(t *testing.T) {
	svc, stub := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1") // pulled status = "todo"
	_ = svc.ViewMutate("test:PROJ-1", "status", "done")

	// Simulate a concurrent remote change to the SAME field (status → in_progress).
	stub.Issue = &domain.Issue{
		Key:       "PROJ-1",
		Title:     "Fix bug",
		Status:    domain.StatusInProgress, // remote changed status independently
		UpdatedAt: time.Date(2026, 5, 11, 15, 0, 0, 0, time.UTC),
	}

	_, err := svc.ViewPush(ctx, "test:PROJ-1")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "remote record changed since pull") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestViewPushAll(t *testing.T) {
	svc, stub := newViewTestService(t)
	ctx := context.Background()

	fixedTime := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	stub.Issue = &domain.Issue{
		Key: "PROJ-2", Title: "Second bug", Status: domain.StatusBacklog,
		UpdatedAt: fixedTime,
	}
	_, _ = svc.ViewPull(ctx, "test:PROJ-2")

	_ = svc.ViewMutate("test:PROJ-1", "status", "done")
	_ = svc.ViewMutate("test:PROJ-2", "assignee", "bob")
	// Keep stub at pulled state so field-level conflict check finds no remote changes.
	stub.Issue = &domain.Issue{
		Key: "PROJ-1", Title: "Fix bug", Status: domain.StatusTodo,
		UpdatedAt: fixedTime,
	}

	pushed, errs := svc.ViewPushAll(ctx)
	if len(errs) != 0 {
		t.Errorf("push_all errors: %v", errs)
	}
	if len(pushed) != 2 {
		t.Errorf("expected 2 pushed, got %d", len(pushed))
	}

	dirty := svc.ViewDirty()
	if len(dirty) != 0 {
		t.Errorf("expected clean after push_all, got %d dirty", len(dirty))
	}
}

func TestViewReset(t *testing.T) {
	svc, _ := newViewTestService(t)
	ctx := context.Background()

	_, _ = svc.ViewPull(ctx, "test:PROJ-1")
	_ = svc.ViewMutate("test:PROJ-1", "status", "done")

	svc.ViewReset()

	list := svc.ViewList()
	if len(list) != 0 {
		t.Errorf("expected empty list after reset, got %d", len(list))
	}
	dirty := svc.ViewDirty()
	if len(dirty) != 0 {
		t.Errorf("expected no dirty after reset, got %d", len(dirty))
	}
}
