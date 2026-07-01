package jira_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/infrastructure/jira"
)

var lastCreateBody map[string]any //nolint:gochecknoglobals // test spy
var lastUpdateBody map[string]any //nolint:gochecknoglobals // test spy

// fakeJira serves a mock Jira REST API.
//
//nolint:gocyclo // mock server handling many endpoints
func fakeJira() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		// Get issue
		case r.Method == "GET" && strings.HasPrefix(path, "/rest/api/2/issue/") && !strings.HasSuffix(path, "/transitions"):
			key := strings.TrimPrefix(path, "/rest/api/2/issue/")
			key = strings.Split(key, "?")[0]
			if key == "NOTFOUND-999" {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(jiraIssue(key, "Issue "+key, "New", "new", "Major", "Alice"))

		// Get transitions
		case r.Method == "GET" && strings.HasSuffix(path, "/transitions"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{
					{"id": "11", "name": "New"},
					{"id": "21", "name": "IN_PROGRESS"},
					{"id": "31", "name": "ON_QA"},
					{"id": "41", "name": "Verified"},
					{"id": "51", "name": "Closed"},
				},
			})

		// Post transition
		case r.Method == "POST" && strings.HasSuffix(path, "/transitions"):
			w.WriteHeader(http.StatusNoContent)

		// Create issue
		case r.Method == "POST" && path == "/rest/api/2/issue":
			lastCreateBody = nil
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			lastCreateBody = body
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":  "10001",
				"key": "TEST-99",
			})

		// Update issue
		case r.Method == "PUT" && strings.HasPrefix(path, "/rest/api/2/issue/"):
			lastUpdateBody = nil
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			lastUpdateBody = body
			w.WriteHeader(http.StatusNoContent)

		// Search (v3 API with ADF description)
		case r.Method == "POST" && path == "/rest/api/3/search/jql":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues": []map[string]any{
					jiraIssueADF("TEST-1", "First result", "New", "new", "Critical", "Bob"),
					jiraIssueADF("TEST-2", "Second result", "Done", "done", "Minor", ""),
				},
			})

		// List projects
		case r.Method == "GET" && path == "/rest/api/2/project":
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"id": "10000", "key": "TEST", "name": "Test Project"},
				{"id": "10001", "key": "OPS", "name": "Operations"},
			})

		// List labels
		case r.Method == "GET" && path == "/rest/api/2/label":
			_ = json.NewEncoder(w).Encode([]string{"bug", "feature", "docs"})

		default:
			http.Error(w, "not found: "+path, http.StatusNotFound)
		}
	}))
}

func jiraIssue(key, summary, statusName, statusCategory, priority, assignee string) map[string]any {
	issue := map[string]any{
		"id":   "id-" + key,
		"key":  key,
		"self": "https://jira.example.com/rest/api/2/issue/" + key,
		"fields": map[string]any{
			"summary":     summary,
			"description": "Description for " + key,
			"status": map[string]any{
				"name":           statusName,
				"statusCategory": map[string]string{"key": statusCategory},
			},
			"priority": map[string]string{"name": priority},
			"labels":   []string{"bug", "telco"},
			"project":  map[string]string{"key": "TEST", "name": "Test Project"},
			"created":  "2025-01-15T10:30:00.000+0000",
			"updated":  "2025-01-16T14:00:00.000+0000",
		},
	}
	if assignee != "" {
		issue["fields"].(map[string]any)["assignee"] = map[string]string{"displayName": assignee}
	}
	return issue
}

func jiraIssueADF(key, summary, statusName, statusCategory, priority, assignee string) map[string]any {
	issue := jiraIssue(key, summary, statusName, statusCategory, priority, assignee)
	// v3 API returns description as ADF object
	issue["fields"].(map[string]any)["description"] = map[string]any{
		"type": "doc",
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": "ADF description for " + key},
				},
			},
		},
	}
	return issue
}

func newTestRepo(t *testing.T) (*jira.Repository, *httptest.Server) {
	t.Helper()
	srv := fakeJira()
	repo, err := jira.New("jira", srv.URL, "test@example.com", "test-token", "TEST", nil)
	if err != nil {
		srv.Close()
		t.Fatalf("New: %v", err)
	}
	return repo, srv
}

func TestE2E_Get(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issue, err := repo.Get(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Ref != "jira:TEST-1" {
		t.Errorf("ref = %q, want jira:TEST-1", issue.Ref)
	}
	if issue.Title != "Issue TEST-1" {
		t.Errorf("title = %q, want %q", issue.Title, "Issue TEST-1")
	}
	if issue.Assignee != "Alice" {
		t.Errorf("assignee = %q, want Alice", issue.Assignee)
	}
	if issue.Project != "TEST" {
		t.Errorf("project = %q, want TEST", issue.Project)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("labels = %v, want [bug telco]", issue.Labels)
	}
	if issue.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestE2E_GetNotFound(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	_, err := repo.Get(context.Background(), "NOTFOUND-999")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
}

func TestE2E_Create(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issue, err := repo.Create(context.Background(), domain.CreateInput{
		Title:    "New issue",
		Priority: domain.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// After create, it fetches the issue via Get
	if issue.Key != "TEST-99" {
		t.Errorf("key = %q, want TEST-99", issue.Key)
	}
}

func TestE2E_CreateWithParent(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	_, err := repo.Create(context.Background(), domain.CreateInput{
		Title:    "Child task",
		ParentID: "PROJ-42",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if lastCreateBody == nil {
		t.Fatal("create body not captured")
	}
	fields, _ := lastCreateBody["fields"].(map[string]any)
	parent, ok := fields["parent"].(map[string]any)
	if !ok {
		t.Fatal("fields.parent not set in API request")
	}
	if parent["key"] != "PROJ-42" {
		t.Errorf("parent.key = %v, want PROJ-42", parent["key"])
	}
}

func TestE2E_CreateNoProject(t *testing.T) {
	srv := fakeJira()
	defer srv.Close()

	repo, _ := jira.New("jira", srv.URL, "test@example.com", "test-token", "", nil)
	_, err := repo.Create(context.Background(), domain.CreateInput{Title: "No project"})
	if err == nil {
		t.Fatal("expected error when no project configured")
	}
}

func TestE2E_Update(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	newTitle := "Updated title"
	issue, err := repo.Update(context.Background(), "TEST-1", domain.UpdateInput{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if issue.Ref != "jira:TEST-1" {
		t.Errorf("ref = %q, want jira:TEST-1", issue.Ref)
	}
}

func TestE2E_UpdateParent(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	parentKey := "PROJ-10"
	_, err := repo.Update(context.Background(), "TEST-1", domain.UpdateInput{
		ParentID: &parentKey,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if lastUpdateBody == nil {
		t.Fatal("update body not captured")
	}
	fields, _ := lastUpdateBody["fields"].(map[string]any)
	parent, ok := fields["parent"].(map[string]any)
	if !ok {
		t.Fatal("fields.parent not set in API request")
	}
	if parent["key"] != "PROJ-10" {
		t.Errorf("parent.key = %v, want PROJ-10", parent["key"])
	}
}

func TestE2E_UpdateStatus(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	status := domain.StatusDone
	issue, err := repo.Update(context.Background(), "TEST-1", domain.UpdateInput{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("Update with status: %v", err)
	}
	if issue.Ref != "jira:TEST-1" {
		t.Errorf("ref = %q, want jira:TEST-1", issue.Ref)
	}
}

func TestE2E_Search(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.Search(context.Background(), "test query", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}
	if issues[0].Ref != "jira:TEST-1" {
		t.Errorf("ref = %q, want jira:TEST-1", issues[0].Ref)
	}
}

func TestE2E_SearchADFDescription(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	issues, err := repo.Search(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// v3 search returns ADF — should be extracted to plain text
	if !strings.Contains(issues[0].Description, "ADF description for TEST-1") {
		t.Errorf("description = %q, want to contain 'ADF description for TEST-1'", issues[0].Description)
	}
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
}

func TestE2E_ListChildren(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	children, err := repo.ListChildren(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	// Uses search API, returns 2 results from our mock
	if len(children) != 2 {
		t.Fatalf("got %d children, want 2", len(children))
	}
}

func TestE2E_ListProjects(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	projs, err := repo.ListProjects(context.Background(), domain.ProjectListFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projs) != 2 {
		t.Fatalf("got %d projects, want 2", len(projs))
	}
	if projs[0].ID != "TEST" {
		t.Errorf("id = %q, want TEST", projs[0].ID)
	}
}

func TestE2E_ListLabels(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	labels, err := repo.ListLabels(context.Background())
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) != 3 {
		t.Fatalf("got %d labels, want 3", len(labels))
	}
	if labels[0].Name != "bug" {
		t.Errorf("name = %q, want bug", labels[0].Name)
	}
}

func TestE2E_StatusMapping(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	// "new" category -> todo
	issue, err := repo.Get(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Status != domain.StatusTodo {
		t.Errorf("status = %q, want todo", issue.Status)
	}
}

func TestE2E_PriorityMapping(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	// "Major" -> high
	issue, err := repo.Get(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Priority != domain.PriorityHigh {
		t.Errorf("priority = %d, want %d (high)", issue.Priority, domain.PriorityHigh)
	}
}

func makeLargeFieldManifest(n int) map[string]string {
	cf := make(map[string]string, n)
	for i := range n {
		cf[fmt.Sprintf("Custom Field %d", i)] = fmt.Sprintf("customfield_%d", 10000+i)
	}
	return cf
}

func newTestRepoWithFields(t *testing.T, srv *httptest.Server, cf map[string]string) *jira.Repository {
	t.Helper()
	repo, err := jira.New("jira", srv.URL, "u@x.com", "tok", "TEST", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	repo.SetCustomFields(cf)
	return repo
}

func TestE2E_GetNoFieldsInURL(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			capturedURL = r.URL.String()
			_ = json.NewEncoder(w).Encode(jiraIssue("TEST-1", "Test", "New", "new", "Major", ""))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	repo := newTestRepoWithFields(t, srv, makeLargeFieldManifest(600))

	_, err := repo.Get(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if strings.Contains(capturedURL, "fields=") {
		t.Errorf("GET URL contains fields= param (len=%d), should not include fields to avoid 414", len(capturedURL))
	}
	if len(capturedURL) > 200 {
		t.Errorf("GET URL unexpectedly long: %d chars", len(capturedURL))
	}
}

func TestE2E_SearchUsesPostWithFields(t *testing.T) {
	var (
		capturedMethod string
		capturedBody   struct {
			JQL        string   `json:"jql"`
			MaxResults int      `json:"maxResults"`
			Fields     []string `json:"fields"`
		}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/3/search/jql" {
			capturedMethod = r.Method
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []any{}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	repo := newTestRepoWithFields(t, srv, makeLargeFieldManifest(600))

	_, err := repo.Search(context.Background(), "test query", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if capturedMethod != "POST" {
		t.Errorf("search method = %q, want POST", capturedMethod)
	}
	if capturedBody.JQL == "" {
		t.Error("search body missing jql")
	}
	if capturedBody.MaxResults != 10 {
		t.Errorf("search body maxResults = %d, want 10", capturedBody.MaxResults)
	}
	if len(capturedBody.Fields) < 600 {
		t.Errorf("search body fields count = %d, want >= 600", len(capturedBody.Fields))
	}
}

func TestE2E_GetWithCustomFieldExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			issue := jiraIssue("TEST-1", "Test", "New", "new", "Major", "")
			fields := issue["fields"].(map[string]any)
			fields["customfield_10020"] = []map[string]any{
				{"name": "Sprint 42", "state": "active"},
			}
			fields["customfield_10030"] = 5.0
			_ = json.NewEncoder(w).Encode(issue)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	repo := newTestRepoWithFields(t, srv, map[string]string{
		"Sprint":       "customfield_10020",
		"Story Points": "customfield_10030",
	})

	issue, err := repo.Get(context.Background(), "TEST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Sprint != "Sprint 42 (active)" {
		t.Errorf("sprint = %q, want %q", issue.Sprint, "Sprint 42 (active)")
	}
	if issue.StoryPoints == nil || *issue.StoryPoints != 5.0 {
		t.Errorf("story points = %v, want 5.0", issue.StoryPoints)
	}
}

func TestE2E_UnsupportedOps(t *testing.T) {
	repo, srv := newTestRepo(t)
	defer srv.Close()

	_, err := repo.CreateProject(context.Background(), domain.ProjectCreateInput{Name: "test"})
	if err == nil {
		t.Error("CreateProject should return error")
	}

	_, err = repo.CreateLabel(context.Background(), domain.LabelCreateInput{Name: "test"})
	if err == nil {
		t.Error("CreateLabel should return error")
	}
}
