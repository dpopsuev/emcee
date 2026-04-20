package github

import (
	"os"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

func init() {
	adapterdriven.Register("github", 0, func(name string, backend config.Backend) (driven.IssueRepository, error) {
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
