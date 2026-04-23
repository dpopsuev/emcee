package driventest_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven/driventest"
)

func TestStubIssueRepository_Name(t *testing.T) {
	stub := driventest.NewStubIssueRepository("linear")
	if stub.Name() != "linear" {
		t.Errorf("Name() = %q, want %q", stub.Name(), "linear")
	}
}

//nolint:gocyclo // comprehensive spy verification test
func TestStubIssueRepository_SpyRecording(t *testing.T) {
	stub := driventest.NewStubIssueRepository("test")
	stub.Issues = []domain.Issue{{Key: "T-1", Title: "First"}}
	stub.Issue = &domain.Issue{Key: "T-1", Title: "First"}
	ctx := context.Background()

	// List
	issues, err := stub.List(ctx, domain.ListFilter{Status: domain.StatusTodo})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("List returned %d issues, want 1", len(issues))
	}
	if len(stub.ListCalls) != 1 {
		t.Fatalf("ListCalls = %d, want 1", len(stub.ListCalls))
	}
	if stub.ListCalls[0].Filter.Status != domain.StatusTodo {
		t.Errorf("ListCalls[0].Filter.Status = %q, want %q", stub.ListCalls[0].Filter.Status, domain.StatusTodo)
	}

	// Get
	issue, err := stub.Get(ctx, "T-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Title != "First" {
		t.Errorf("Get returned title %q, want %q", issue.Title, "First")
	}
	if len(stub.GetCalls) != 1 || stub.GetCalls[0].Key != "T-1" {
		t.Errorf("GetCalls = %+v, want [{Key: T-1}]", stub.GetCalls)
	}

	// Create
	_, err = stub.Create(ctx, domain.CreateInput{Title: "New"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(stub.CreateCalls) != 1 || stub.CreateCalls[0].Input.Title != "New" {
		t.Errorf("CreateCalls = %+v", stub.CreateCalls)
	}

	// Update
	title := "Updated"
	_, err = stub.Update(ctx, "T-1", domain.UpdateInput{Title: &title})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(stub.UpdateCalls) != 1 || stub.UpdateCalls[0].Key != "T-1" {
		t.Errorf("UpdateCalls = %+v", stub.UpdateCalls)
	}

	// Search
	_, err = stub.Search(ctx, "first", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(stub.SearchCalls) != 1 || stub.SearchCalls[0].Query != "first" || stub.SearchCalls[0].Limit != 10 {
		t.Errorf("SearchCalls = %+v", stub.SearchCalls)
	}

	// ListChildren
	_, err = stub.ListChildren(ctx, "T-1")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(stub.ChildrenCalls) != 1 || stub.ChildrenCalls[0].Key != "T-1" {
		t.Errorf("ChildrenCalls = %+v", stub.ChildrenCalls)
	}
}

func TestStubIssueRepository_ErrorInjection(t *testing.T) {
	stub := driventest.NewStubIssueRepository("test")
	stub.Err = errors.New("global error")
	ctx := context.Background()

	_, err := stub.List(ctx, domain.ListFilter{})
	if err == nil || err.Error() != "global error" {
		t.Errorf("List error = %v, want global error", err)
	}

	// Per-method error takes precedence
	stub.ListErr = errors.New("list error")
	_, err = stub.List(ctx, domain.ListFilter{})
	if err == nil || err.Error() != "list error" {
		t.Errorf("List error = %v, want list error", err)
	}
}

func TestStubIssueRepository_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	stub := driventest.NewStubIssueRepository("test")
	stub.Issues = []domain.Issue{{Key: "T-1"}}
	stub.Issue = &domain.Issue{Key: "T-1"}
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = stub.List(ctx, domain.ListFilter{})
			_, _ = stub.Get(ctx, "T-1")
			_, _ = stub.Create(ctx, domain.CreateInput{Title: "x"})
		}()
	}
	wg.Wait()

	if len(stub.ListCalls) != 50 {
		t.Errorf("ListCalls = %d, want 50", len(stub.ListCalls))
	}
	if len(stub.GetCalls) != 50 {
		t.Errorf("GetCalls = %d, want 50", len(stub.GetCalls))
	}
	if len(stub.CreateCalls) != 50 {
		t.Errorf("CreateCalls = %d, want 50", len(stub.CreateCalls))
	}
}
