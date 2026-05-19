package infrastructure_test

import (
	"errors"
	"testing"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
	"github.com/dpopsuev/emcee/internal/repository/stub"
)

func TestRegistry_RegisterAndCreateFromConfig(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("fake", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return stub.NewStubIssueRepository(name), nil
	})

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"fake": {},
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(repos) != 1 {
		t.Fatalf("repos = %d, want 1", len(repos))
	}
	if repos[0].Name() != "fake" {
		t.Errorf("repo name = %q, want %q", repos[0].Name(), "fake")
	}
}

func TestRegistry_UnknownBackendWarning(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"nonexistent": {},
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(repos) != 0 {
		t.Errorf("repos = %d, want 0", len(repos))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(warnings))
	}
	if warnings[0] == "" {
		t.Error("expected non-empty warning")
	}
}

func TestRegistry_FactoryError(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("broken", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, errors.New("bad config")
	})

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"broken": {},
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(repos) != 0 {
		t.Errorf("repos = %d, want 0", len(repos))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(warnings))
	}
}

func TestRegistry_FactoryReturnsNil(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("optional", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, nil // not applicable
	})

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"optional": {},
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(repos) != 0 {
		t.Errorf("repos = %d, want 0", len(repos))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}

func TestRegistry_CreateFromEnv(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("env-backend", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return stub.NewStubIssueRepository(name), nil
	})
	infra.Register("skip-backend", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, nil // not applicable
	})

	repos, warnings := infra.CreateFromEnv()
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(repos) != 1 {
		t.Fatalf("repos = %d, want 1", len(repos))
	}
	if repos[0].Name() != "env-backend" {
		t.Errorf("repo name = %q, want %q", repos[0].Name(), "env-backend")
	}
}

func TestRegistry_Available(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("alpha", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, nil
	})
	infra.Register("beta", 10, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, nil
	})

	names := infra.Available()
	if len(names) != 2 {
		t.Fatalf("available = %d, want 2", len(names))
	}
	// Higher priority first
	if names[0] != "beta" {
		t.Errorf("first = %q, want %q (higher priority)", names[0], "beta")
	}
}

func TestRegistry_MultiInstanceSameType(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("jenkins", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return stub.NewStubIssueRepository(name), nil
	})

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"jenkins-ci":   {Type: "jenkins"},
			"jenkins-auto": {Type: "jenkins"},
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(repos) != 2 {
		t.Fatalf("repos = %d, want 2", len(repos))
	}

	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name()] = true
	}
	if !names["jenkins-ci"] {
		t.Error("missing jenkins-ci instance")
	}
	if !names["jenkins-auto"] {
		t.Error("missing jenkins-auto instance")
	}
}

func TestRegistry_TypeInferredFromKey(t *testing.T) {
	infra.Reset()
	t.Cleanup(infra.Reset)

	infra.Register("jira", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return stub.NewStubIssueRepository(name), nil
	})

	cfg := &config.Config{
		Backends: map[string]config.Backend{
			"jira": {}, // no Type field — inferred from key
		},
	}
	repos, warnings := infra.CreateFromConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(repos) != 1 {
		t.Fatalf("repos = %d, want 1", len(repos))
	}
	if repos[0].Name() != "jira" {
		t.Errorf("name = %q, want %q", repos[0].Name(), "jira")
	}
}

func TestRegistry_Reset(t *testing.T) {
	infra.Reset()
	infra.Register("test", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		return nil, nil
	})
	infra.Reset()
	names := infra.Available()
	if len(names) != 0 {
		t.Errorf("after reset, available = %d, want 0", len(names))
	}
}
