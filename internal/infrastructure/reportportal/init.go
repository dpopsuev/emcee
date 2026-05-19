package reportportal

import (
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

func init() {
	infra.Register("reportportal", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("RP_API_KEY")
		}
		if token == "" {
			return nil, nil
		}
		url := backend.URL
		if url == "" {
			url = os.Getenv("RP_URL")
		}
		if url == "" {
			return nil, nil
		}
		project := backend.Team
		if project == "" {
			project = os.Getenv("RP_PROJECT")
		}
		if project == "" {
			return nil, nil
		}
		return New(name, url, project, token)
	})
}
