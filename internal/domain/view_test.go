package domain_test

import (
	"testing"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
)

func TestViewRecord_FieldAccess(t *testing.T) {
	t.Helper()
	vr := domain.ViewRecord{
		Ref: "jira:PROJ-1",
		Fields: map[string]string{
			"title":  "Fix bug",
			"status": "todo",
		},
		Version:  "2026-01-01T00:00:00Z",
		PulledAt: time.Now(),
	}

	if vr.Fields["title"] != "Fix bug" {
		t.Errorf("expected title 'Fix bug', got %q", vr.Fields["title"])
	}
	if vr.Fields["status"] != "todo" {
		t.Errorf("expected status 'todo', got %q", vr.Fields["status"])
	}
	if _, ok := vr.Fields["missing"]; ok {
		t.Error("expected missing field to be absent")
	}
}

func TestChangeSet_IsDirty(t *testing.T) {
	t.Helper()
	cs := &domain.ChangeSet{Ref: "jira:PROJ-1"}
	if cs.IsDirty() {
		t.Error("empty change set should not be dirty")
	}

	cs.Changes = append(cs.Changes, domain.FieldChange{
		Field:    "status",
		OldValue: "todo",
		NewValue: "in_progress",
	})
	if !cs.IsDirty() {
		t.Error("change set with changes should be dirty")
	}
}

func TestChangeSet_DirtyFields(t *testing.T) {
	t.Helper()
	cs := &domain.ChangeSet{
		Ref: "jira:PROJ-1",
		Changes: []domain.FieldChange{
			{Field: "status", OldValue: "todo", NewValue: "done"},
			{Field: "assignee", OldValue: "", NewValue: "alice"},
		},
	}

	fields := cs.DirtyFields()
	if len(fields) != 2 {
		t.Fatalf("expected 2 dirty fields, got %d", len(fields))
	}
	if fields[0] != "status" || fields[1] != "assignee" {
		t.Errorf("unexpected dirty fields: %v", fields)
	}
}

func TestDirtyTracker_MarkAndGet(t *testing.T) {
	t.Helper()
	dt := domain.NewDirtyTracker()

	if dt.IsDirty() {
		t.Error("new tracker should not be dirty")
	}

	dt.Mark("jira:PROJ-1", "status", "todo", "done")
	dt.Mark("jira:PROJ-1", "assignee", "", "alice")
	dt.Mark("github:42", "title", "old", "new")

	if !dt.IsDirty() {
		t.Error("tracker with marks should be dirty")
	}

	cs := dt.Get("jira:PROJ-1")
	if cs == nil {
		t.Fatal("expected change set for jira:PROJ-1")
	}
	if len(cs.Changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(cs.Changes))
	}

	cs2 := dt.Get("github:42")
	if cs2 == nil || len(cs2.Changes) != 1 {
		t.Fatal("expected 1 change for github:42")
	}

	if dt.Get("nonexistent") != nil {
		t.Error("expected nil for non-tracked ref")
	}
}

func TestDirtyTracker_All(t *testing.T) {
	t.Helper()
	dt := domain.NewDirtyTracker()
	dt.Mark("a:1", "f", "x", "y")
	dt.Mark("b:2", "g", "m", "n")

	all := dt.All()
	if len(all) != 2 {
		t.Errorf("expected 2 change sets, got %d", len(all))
	}
}

func TestDirtyTracker_Clear(t *testing.T) {
	t.Helper()
	dt := domain.NewDirtyTracker()
	dt.Mark("a:1", "f", "x", "y")
	dt.Mark("b:2", "g", "m", "n")

	dt.Clear("a:1")
	if dt.Get("a:1") != nil {
		t.Error("a:1 should be cleared")
	}
	if dt.Get("b:2") == nil {
		t.Error("b:2 should still be tracked")
	}
}

func TestDirtyTracker_ClearAll(t *testing.T) {
	t.Helper()
	dt := domain.NewDirtyTracker()
	dt.Mark("a:1", "f", "x", "y")
	dt.Mark("b:2", "g", "m", "n")

	dt.ClearAll()
	if dt.IsDirty() {
		t.Error("tracker should be clean after ClearAll")
	}
}

func TestIssueToViewRecord(t *testing.T) {
	t.Helper()
	issue := &domain.Issue{
		Title:       "Fix auth",
		Description: "Auth is broken",
		Status:      domain.StatusInProgress,
		Priority:    domain.PriorityHigh,
		Assignee:    "bob",
		URL:         "https://jira.example.com/PROJ-1",
		UpdatedAt:   time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
	}

	vr := domain.IssueToViewRecord("jira:PROJ-1", issue)

	if vr.Ref != "jira:PROJ-1" {
		t.Errorf("expected ref 'jira:PROJ-1', got %q", vr.Ref)
	}
	if vr.Fields["title"] != "Fix auth" {
		t.Errorf("expected title 'Fix auth', got %q", vr.Fields["title"])
	}
	if vr.Fields["status"] != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", vr.Fields["status"])
	}
	if vr.Fields["priority"] != "high" {
		t.Errorf("expected priority 'high', got %q", vr.Fields["priority"])
	}
	if vr.Version != "2026-05-10T12:00:00Z" {
		t.Errorf("expected version '2026-05-10T12:00:00Z', got %q", vr.Version)
	}
	if vr.PulledAt.IsZero() {
		t.Error("PulledAt should not be zero")
	}
}

func TestViewDiff_Structure(t *testing.T) {
	t.Helper()
	diff := domain.ViewDiff{
		Ref: "jira:PROJ-1",
		Changes: []domain.FieldDiff{
			{Field: "status", LocalValue: "done", PullValue: "todo"},
			{Field: "assignee", LocalValue: "alice", PullValue: ""},
		},
	}

	if len(diff.Changes) != 2 {
		t.Errorf("expected 2 field diffs, got %d", len(diff.Changes))
	}
	if diff.Changes[0].Field != "status" {
		t.Errorf("expected first diff field 'status', got %q", diff.Changes[0].Field)
	}
}
