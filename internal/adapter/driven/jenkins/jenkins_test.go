package jenkins

import (
	"context"
	"errors"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// Compile-time interface assertions.
var (
	_ driven.IssueRepository = (*Repository)(nil)
	_ driven.BuildRepository = (*Repository)(nil)
)

func TestIssueRepositoryStubsReturnError(t *testing.T) {
	repo := &Repository{name: BackendName}
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"List", func() error { _, err := repo.List(ctx, domain.ListFilter{}); return err }},
		{"Get", func() error { _, err := repo.Get(ctx, "1"); return err }},
		{"Create", func() error { _, err := repo.Create(ctx, domain.CreateInput{}); return err }},
		{"Update", func() error { _, err := repo.Update(ctx, "1", domain.UpdateInput{}); return err }},
		{"Search", func() error { _, err := repo.Search(ctx, "q", 10); return err }},
		{"ListChildren", func() error { _, err := repo.ListChildren(ctx, "1"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, ErrNotIssueBackend) {
				t.Errorf("got %v, want ErrNotIssueBackend", err)
			}
		})
	}
}

func TestBackendName(t *testing.T) {
	repo := &Repository{name: BackendName}
	if repo.Name() != BackendName {
		t.Errorf("Name() = %q, want %q", repo.Name(), BackendName)
	}
}
