package gitlab

import (
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

func init() {
	infra.Register("gitlab", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("GITLAB_TOKEN")
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
