package linear_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven/linear"
	"github.com/DanyPops/emcee/internal/domain"
)

// fakeLinear serves a mock Linear GraphQL API.
func fakeLinear() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		q := body.Query

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(q, "teams"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"teams": map[string]any{
					"nodes": []map[string]string{
						{"id": "team-1", "key": "TST"},
					},
				},
			}))

		case strings.Contains(q, "searchIssues"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"searchIssues": map[string]any{
					"nodes": []map[string]any{issueNode("TST-1", "Search result", "unstarted", 2)},
				},
			}))

		case strings.Contains(q, "issueCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issueCreate": map[string]any{
					"success": true,
					"issue":   issueNode("TST-99", "Created issue", "backlog", 3),
				},
			}))

		case strings.Contains(q, "issueUpdate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issueUpdate": map[string]any{
					"success": true,
					"issue":   issueNode("TST-1", "Updated title", "started", 2),
				},
			}))

		case strings.Contains(q, "issueBatchCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issueBatchCreate": map[string]any{
					"success": true,
					"issues": []map[string]any{
						issueNode("TST-100", "Bulk 1", "backlog", 0),
						issueNode("TST-101", "Bulk 2", "backlog", 0),
					},
				},
			}))

		case strings.Contains(q, "children"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issues": map[string]any{
					"nodes": []map[string]any{
						{
							"children": map[string]any{
								"nodes": []map[string]any{
									issueNode("TST-10", "Child issue", "unstarted", 0),
								},
							},
						},
					},
				},
			}))

		case strings.Contains(q, "states"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"team": map[string]any{
					"states": map[string]any{
						"nodes": []map[string]string{
							{"id": "state-1", "type": "backlog"},
							{"id": "state-2", "type": "unstarted"},
							{"id": "state-3", "type": "started"},
							{"id": "state-4", "type": "completed"},
							{"id": "state-5", "type": "canceled"},
						},
					},
				},
			}))

		case strings.Contains(q, "issues"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issues": map[string]any{
					"nodes": []map[string]any{
						issueNode("TST-1", "First issue", "unstarted", 2),
						issueNode("TST-2", "Second issue", "completed", 4),
					},
				},
			}))

		case strings.Contains(q, "documents"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"documents": map[string]any{
					"nodes": []map[string]any{
						{"id": "doc-1", "title": "Design doc", "content": "body", "url": "https://linear.app/doc/1", "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-01-01T00:00:00Z"},
					},
				},
			}))

		case strings.Contains(q, "documentCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"documentCreate": map[string]any{
					"success":  true,
					"document": map[string]any{"id": "doc-2", "title": "New doc", "content": "", "url": "", "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-01-01T00:00:00Z"},
				},
			}))

		case strings.Contains(q, "projectCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"projectCreate": map[string]any{
					"success": true,
					"project": map[string]any{"id": "proj-1", "name": "New project", "state": "planned", "url": "https://linear.app/proj/1", "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-01-01T00:00:00Z"},
				},
			}))

		case strings.Contains(q, "projects"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"projects": map[string]any{
					"nodes": []map[string]any{
						{"id": "proj-1", "name": "Project One", "state": "started", "url": "https://linear.app/proj/1", "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-01-01T00:00:00Z"},
					},
				},
			}))

		case strings.Contains(q, "initiativeCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"initiativeCreate": map[string]any{
					"success":    true,
					"initiative": map[string]any{"id": "init-1", "name": "New initiative", "status": "planned"},
				},
			}))

		case strings.Contains(q, "initiatives"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"initiatives": map[string]any{
					"nodes": []map[string]any{
						{"id": "init-1", "name": "Initiative One", "status": "started"},
					},
				},
			}))

		case strings.Contains(q, "issueLabelCreate"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issueLabelCreate": map[string]any{
					"success":    true,
					"issueLabel": map[string]any{"id": "label-1", "name": "bug", "color": "#ff0000"},
				},
			}))

		case strings.Contains(q, "issueLabels"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"issueLabels": map[string]any{
					"nodes": []map[string]any{
						{"id": "label-1", "name": "bug", "color": "#ff0000"},
						{"id": "label-2", "name": "feature", "color": "#00ff00"},
					},
				},
			}))

		case strings.Contains(q, "users"):
			json.NewEncoder(w).Encode(gqlResp(map[string]any{
				"users": map[string]any{
					"nodes": []map[string]any{
						{"id": "user-1", "name": "Alice"},
					},
				},
			}))

		default:
			json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "unhandled query"}},
			})
		}
	}))
}

func issueNode(identifier, title, stateType string, priority int) map[string]any {
	return map[string]any{
		"id":          "id-" + identifier,
		"identifier":  identifier,
		"title":       title,
		"description": "desc for " + identifier,
		"priority":    priority,
		"url":         "https://linear.app/issue/" + identifier,
		"createdAt":   "2025-01-01T00:00:00Z",
		"updatedAt":   "2025-01-01T00:00:00Z",
		"state":       map[string]string{"name": stateType, "type": stateType},
		"assignee":    map[string]string{"name": "Alice"},
		"labels":      map[string]any{"nodes": []map[string]string{{"name": "bug"}}},
	}
}

func gqlResp(data any) map[string]any {
	return map[string]any{"data": data}
}

func newTestRepo(t *testing.T) (*linear.Repository, *httptest.Server) {
	t.Helper()
	srv := fakeLinear()
	repo, err := linear.NewWithURL("linear", "test-key", "TST", srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("NewWithURL: %v", err)
	}
	return repo, srv
}

func TestE2E_List(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.List(context.Background(), domain.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}
	if issues[0].Ref != "linear:TST-1" {
		t.Errorf("ref = %q, want linear:TST-1", issues[0].Ref)
	}
	if issues[0].Assignee != "Alice" {
		t.Errorf("assignee = %q, want Alice", issues[0].Assignee)
	}
	if len(issues[0].Labels) != 1 || issues[0].Labels[0] != "bug" {
		t.Errorf("labels = %v, want [bug]", issues[0].Labels)
	}
}

func TestE2E_Get(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issue, err := repo.Get(context.Background(), "TST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Title != "First issue" {
		t.Errorf("title = %q, want %q", issue.Title, "First issue")
	}
	if issue.Status != domain.StatusTodo {
		t.Errorf("status = %q, want %q", issue.Status, domain.StatusTodo)
	}
	if issue.Priority != domain.PriorityHigh {
		t.Errorf("priority = %d, want %d", issue.Priority, domain.PriorityHigh)
	}
}

func TestE2E_Create(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issue, err := repo.Create(context.Background(), domain.CreateInput{
		Title:    "Created issue",
		Priority: domain.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if issue.Key != "TST-99" {
		t.Errorf("key = %q, want TST-99", issue.Key)
	}
	if issue.Status != domain.StatusBacklog {
		t.Errorf("status = %q, want %q", issue.Status, domain.StatusBacklog)
	}
}

func TestE2E_Search(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.Search(context.Background(), "search", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Title != "Search result" {
		t.Errorf("title = %q, want %q", issues[0].Title, "Search result")
	}
}

func TestE2E_ListChildren(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	children, err := repo.ListChildren(context.Background(), "TST-1")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("got %d children, want 1", len(children))
	}
	if children[0].Key != "TST-10" {
		t.Errorf("key = %q, want TST-10", children[0].Key)
	}
}

func TestE2E_BulkCreate(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.BulkCreateIssues(context.Background(), []domain.CreateInput{
		{Title: "Bulk 1"},
		{Title: "Bulk 2"},
	})
	if err != nil {
		t.Fatalf("BulkCreateIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("got %d issues, want 2", len(issues))
	}
}

func TestE2E_ListDocuments(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	docs, err := repo.ListDocuments(context.Background(), domain.DocumentListFilter{})
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Design doc" {
		t.Errorf("docs = %v", docs)
	}
}

func TestE2E_CreateDocument(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	doc, err := repo.CreateDocument(context.Background(), domain.DocumentCreateInput{Title: "New doc"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.Title != "New doc" {
		t.Errorf("title = %q, want %q", doc.Title, "New doc")
	}
}

func TestE2E_ListProjects(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	projs, err := repo.ListProjects(context.Background(), domain.ProjectListFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projs) != 1 || projs[0].Name != "Project One" {
		t.Errorf("projects = %v", projs)
	}
}

func TestE2E_CreateProject(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	proj, err := repo.CreateProject(context.Background(), domain.ProjectCreateInput{Name: "New project"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.Name != "New project" {
		t.Errorf("name = %q, want %q", proj.Name, "New project")
	}
}

func TestE2E_ListInitiatives(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	inits, err := repo.ListInitiatives(context.Background(), domain.InitiativeListFilter{})
	if err != nil {
		t.Fatalf("ListInitiatives: %v", err)
	}
	if len(inits) != 1 || inits[0].Name != "Initiative One" {
		t.Errorf("initiatives = %v", inits)
	}
}

func TestE2E_CreateInitiative(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	init, err := repo.CreateInitiative(context.Background(), domain.InitiativeCreateInput{Name: "New initiative"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if init.Name != "New initiative" {
		t.Errorf("name = %q, want %q", init.Name, "New initiative")
	}
}

func TestE2E_ListLabels(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	labels, err := repo.ListLabels(context.Background())
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("got %d labels, want 2", len(labels))
	}
}

func TestE2E_CreateLabel(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	label, err := repo.CreateLabel(context.Background(), domain.LabelCreateInput{Name: "bug", Color: "#ff0000"})
	if err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if label.Name != "bug" {
		t.Errorf("name = %q, want %q", label.Name, "bug")
	}
}

func TestE2E_StatusMapping(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.List(context.Background(), domain.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// TST-1: unstarted -> todo, TST-2: completed -> done
	if issues[0].Status != domain.StatusTodo {
		t.Errorf("TST-1 status = %q, want todo", issues[0].Status)
	}
	if issues[1].Status != domain.StatusDone {
		t.Errorf("TST-2 status = %q, want done", issues[1].Status)
	}
}
