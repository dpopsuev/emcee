package jira

import (
	"context"
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	"github.com/dpopsuev/emcee/internal/fieldmanifest"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/poller"
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
		repo, err := New(name, url, email, token, project, manifest.Mappings)
		if err != nil {
			return nil, err
		}

		// Register a poller so serveCmd can keep the manifest evergreen.
		// The closure captures repo before cache.New() wraps it, so SetCustomFields
		// reaches the live Repository directly.
		poller.Register("fields:"+name, fieldmanifest.NewManifestPoller(
			name,
			config.Dir(),
			fieldmanifest.DefaultTTL,
			func(ctx context.Context) ([]fieldmanifest.NamedField, error) {
				domainFields, err := repo.ListFields(ctx)
				if err != nil {
					return nil, err
				}
				named := make([]fieldmanifest.NamedField, len(domainFields))
				for i, f := range domainFields {
					named[i] = fieldmanifest.NamedField{ID: f.ID, Name: f.Name, Custom: f.Custom}
				}
				return named, nil
			},
			repo.SetCustomFields,
		))

		return repo, nil
	})
}
