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
		if token == "" {
			return nil, nil
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
		if repoName == "" {
			return nil, nil
		}
		return New(token, owner, repoName)
	})
}
