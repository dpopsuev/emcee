package jira

import (
	"os"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

func init() {
	adapterdriven.Register("jira", 0, func(name string, backend config.Backend) (driven.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("JIRA_API_TOKEN")
		}
		if token == "" {
			return nil, nil
		}
		url := backend.URL
		if url == "" {
			url = os.Getenv("JIRA_URL")
		}
		if url == "" {
			return nil, nil
		}
		email := backend.Email
		if email == "" {
			email = os.Getenv("JIRA_EMAIL")
		}
		if email == "" {
			return nil, nil
		}
		project := backend.Team
		if project == "" {
			project = os.Getenv("JIRA_PROJECT")
		}
		return New(url, email, token, project)
	})
}
