package jira

import (
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	"github.com/dpopsuev/emcee/internal/fieldmanifest"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

func init() {
	infra.Register("jira", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
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
		// Load the field manifest for this backend, then apply any explicit
		// overrides from config.yaml backend.fields on top.
		manifest, err := fieldmanifest.Load(name, config.Dir())
		if err != nil {
			return nil, err
		}
		if len(backend.Fields) > 0 {
			manifest = manifest.Merge(backend.Fields)
		}
		return New(name, url, email, token, project, manifest.Mappings)
	})
}
