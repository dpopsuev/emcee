package application_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/application"
	"github.com/dpopsuev/emcee/internal/domain"
)

func TestStagePatch_ProjectID(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "fix bug"}, "missing project")

	projectID := "MYPROJ"
	item, err := store.StagePatch(id, domain.StagePatchInput{
		ProjectID: &projectID,
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if item.Input.ProjectID != "MYPROJ" {
		t.Errorf("ProjectID = %q, want %q", item.Input.ProjectID, "MYPROJ")
	}
}

func TestStagePatch_ParentID(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "subtask"}, "")

	parentID := "PROJ-42"
	item, err := store.StagePatch(id, domain.StagePatchInput{
		UpdateInput: domain.UpdateInput{ParentID: &parentID},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if item.Input.ParentID != "PROJ-42" {
		t.Errorf("ParentID = %q, want %q", item.Input.ParentID, "PROJ-42")
	}
}

func TestStagePatch_IssueType(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "task"}, "")

	issueType := "Bug"
	item, err := store.StagePatch(id, domain.StagePatchInput{
		IssueType: &issueType,
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if item.Input.IssueType != "Bug" {
		t.Errorf("IssueType = %q, want %q", item.Input.IssueType, "Bug")
	}
}

func TestStagePatch_Versions(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "task"}, "")

	item, err := store.StagePatch(id, domain.StagePatchInput{
		Versions: []string{"4.18", "4.19"},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if len(item.Input.Versions) != 2 || item.Input.Versions[0] != "4.18" {
		t.Errorf("Versions = %v, want [4.18, 4.19]", item.Input.Versions)
	}
}

func TestStagePatch_Components(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "task"}, "")

	item, err := store.StagePatch(id, domain.StagePatchInput{
		UpdateInput: domain.UpdateInput{
			Components: []string{"ptp", "networking"},
		},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if len(item.Input.Components) != 2 || item.Input.Components[0] != "ptp" {
		t.Errorf("Components = %v, want [ptp, networking]", item.Input.Components)
	}
}

func TestStagePatch_FixVersions(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "task"}, "")

	item, err := store.StagePatch(id, domain.StagePatchInput{
		UpdateInput: domain.UpdateInput{
			FixVersions: []string{"4.20"},
		},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if len(item.Input.FixVersions) != 1 || item.Input.FixVersions[0] != "4.20" {
		t.Errorf("FixVersions = %v, want [4.20]", item.Input.FixVersions)
	}
}

func TestStagePatch_AllCreateFields(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{Title: "original"}, "failed")

	title := "updated title"
	desc := "description"
	projectID := "MYPROJ"
	parentID := "MYPROJ-1"
	issueType := "Bug"
	assignee := "alice"
	item, err := store.StagePatch(id, domain.StagePatchInput{
		UpdateInput: domain.UpdateInput{
			Title:       &title,
			Description: &desc,
			Assignee:    &assignee,
			ParentID:    &parentID,
			Components:  []string{"ptp"},
			FixVersions: []string{"4.20"},
		},
		ProjectID: &projectID,
		IssueType: &issueType,
		Versions:  []string{"4.18"},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if item.Input.Title != "updated title" {
		t.Errorf("Title = %q, want %q", item.Input.Title, "updated title")
	}
	if item.Input.Description != "description" {
		t.Errorf("Description = %q, want %q", item.Input.Description, "description")
	}
	if item.Input.ProjectID != "MYPROJ" {
		t.Errorf("ProjectID = %q, want %q", item.Input.ProjectID, "MYPROJ")
	}
	if item.Input.ParentID != "MYPROJ-1" {
		t.Errorf("ParentID = %q, want %q", item.Input.ParentID, "MYPROJ-1")
	}
	if item.Input.IssueType != "Bug" {
		t.Errorf("IssueType = %q, want %q", item.Input.IssueType, "Bug")
	}
	if item.Input.Assignee != "alice" {
		t.Errorf("Assignee = %q, want %q", item.Input.Assignee, "alice")
	}
	if len(item.Input.Versions) != 1 || item.Input.Versions[0] != "4.18" {
		t.Errorf("Versions = %v, want [4.18]", item.Input.Versions)
	}
	if len(item.Input.Components) != 1 || item.Input.Components[0] != "ptp" {
		t.Errorf("Components = %v, want [ptp]", item.Input.Components)
	}
	if len(item.Input.FixVersions) != 1 || item.Input.FixVersions[0] != "4.20" {
		t.Errorf("FixVersions = %v, want [4.20]", item.Input.FixVersions)
	}
}

func TestStagePatch_PreservesExistingFields(t *testing.T) {
	store := application.NewStageStore()
	id := store.StageItem("jira", domain.CreateInput{
		Title:     "original",
		ProjectID: "EXISTING",
		ParentID:  "EXISTING-1",
		IssueType: "Task",
	}, "")

	title := "new title"
	item, err := store.StagePatch(id, domain.StagePatchInput{
		UpdateInput: domain.UpdateInput{Title: &title},
	})
	if err != nil {
		t.Fatalf("StagePatch: %v", err)
	}
	if item.Input.Title != "new title" {
		t.Errorf("Title = %q, want %q", item.Input.Title, "new title")
	}
	if item.Input.ProjectID != "EXISTING" {
		t.Errorf("ProjectID = %q, want %q (should be preserved)", item.Input.ProjectID, "EXISTING")
	}
	if item.Input.ParentID != "EXISTING-1" {
		t.Errorf("ParentID = %q, want %q (should be preserved)", item.Input.ParentID, "EXISTING-1")
	}
	if item.Input.IssueType != "Task" {
		t.Errorf("IssueType = %q, want %q (should be preserved)", item.Input.IssueType, "Task")
	}
}
