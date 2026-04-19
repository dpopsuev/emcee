package gitlab

import (
	"os"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

func init() {
	adapterdriven.Register("gitlab", 0, func(name string, backend config.Backend) (driven.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("GITLAB_TOKEN")
		}
		if token == "" {
			return nil, nil
		}
		project := backend.Team
		if project == "" {
			project = os.Getenv("GITLAB_PROJECT")
		}
		if project == "" {
			return nil, nil
		}
		baseURL := backend.URL
		if baseURL == "" {
			baseURL = os.Getenv("GITLAB_URL")
		}
		if baseURL == "" {
			baseURL = "https://gitlab.com"
		}
		return NewWithURL(name, token, project, baseURL)
	})
}
