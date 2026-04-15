package config

import (
	"os"
	"path/filepath"
	"testing"
)

const validYAML = `backends:
  linear:
    api_key_env: LINEAR_API_KEY
    team: HEG
  github:
    token_env: GITHUB_TOKEN
  jira:
    url: https://mycompany.atlassian.net
    token_env: JIRA_API_TOKEN
    email: me@company.com

projects:
  hegemony:
    backend: linear
    project: HEG
  emcee:
    backend: github
    repo: DanyPops/emcee
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), configFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := writeConfig(t, validYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Backends) != 3 {
		t.Errorf("backends = %d, want 3", len(cfg.Backends))
	}

	lin := cfg.Backends["linear"]
	if lin.APIKeyEnv != "LINEAR_API_KEY" {
		t.Errorf("linear.api_key_env = %q, want LINEAR_API_KEY", lin.APIKeyEnv)
	}
	if lin.Team != "HEG" {
		t.Errorf("linear.team = %q, want HEG", lin.Team)
	}

	jira := cfg.Backends["jira"]
	if jira.URL != "https://mycompany.atlassian.net" {
		t.Errorf("jira.url = %q", jira.URL)
	}
	if jira.Email != "me@company.com" {
		t.Errorf("jira.email = %q", jira.Email)
	}

	if len(cfg.Projects) != 2 {
		t.Errorf("projects = %d, want 2", len(cfg.Projects))
	}

	heg := cfg.Projects["hegemony"]
	if heg.Backend != "linear" || heg.Project != "HEG" {
		t.Errorf("hegemony project = %+v", heg)
	}

	emc := cfg.Projects["emcee"]
	if emc.Backend != "github" || emc.Repo != "DanyPops/emcee" {
		t.Errorf("emcee project = %+v", emc)
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeConfig(t, "{{not yaml}}")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadEmpty(t *testing.T) {
	path := writeConfig(t, "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Backends) != 0 {
		t.Errorf("backends = %d, want 0", len(cfg.Backends))
	}
}

func TestExists(t *testing.T) {
	path := writeConfig(t, validYAML)
	if !Exists(path) {
		t.Error("Exists returned false for existing file")
	}
	if Exists("/nonexistent/path/config.yaml") {
		t.Error("Exists returned true for missing file")
	}
}

func TestResolveKey(t *testing.T) {
	t.Setenv("TEST_EMCEE_KEY", "secret123")

	b := Backend{APIKeyEnv: "TEST_EMCEE_KEY"}
	if got := b.ResolveKey(); got != "secret123" {
		t.Errorf("ResolveKey (api_key_env) = %q, want secret123", got)
	}

	b = Backend{TokenEnv: "TEST_EMCEE_KEY"}
	if got := b.ResolveKey(); got != "secret123" {
		t.Errorf("ResolveKey (token_env) = %q, want secret123", got)
	}

	b = Backend{APIKeyEnv: "TEST_EMCEE_KEY", TokenEnv: "IGNORED"}
	if got := b.ResolveKey(); got != "secret123" {
		t.Errorf("ResolveKey (api_key_env preferred) = %q, want secret123", got)
	}

	b = Backend{}
	if got := b.ResolveKey(); got != "" {
		t.Errorf("ResolveKey (empty) = %q, want empty", got)
	}
}

func TestDefaultPathRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got := defaultPath()
	want := "/custom/config/emcee/config.yaml"
	if got != want {
		t.Errorf("defaultPath = %q, want %q", got, want)
	}
}
