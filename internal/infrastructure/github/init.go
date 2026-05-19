package github

import (
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

func init() {
	infra.Register("github", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		owner := backend.Owner
		if owner == "" {
			owner = os.Getenv("GITHUB_OWNER")
		}
		if owner == "" {
			return nil, nil
		}
		repoName := backend.Team
		if repoName == "" {
			repoName = os.Getenv("GITHUB_REPO")
		}
		return New(name, token, owner, repoName)
	})
}
