package jira

import (
	"context"
	"maps"
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/manifest"
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
		fm, err := manifest.Load(manifest.DefaultKind, name, config.Dir())
		if err != nil {
			return nil, err
		}
		if len(backend.Fields) > 0 {
			fm = fm.Merge(backend.Fields)
		}
		repo, err := New(name, url, email, token, project, fm.Mappings)
		if err != nil {
			return nil, err
		}

		// Load status manifest and apply config overrides.
		sm, err := manifest.Load("statuses", name, config.Dir())
		if err != nil {
			return nil, err
		}
		if len(backend.Statuses) > 0 {
			sm = sm.Merge(backend.Statuses)
		}
		if len(sm.Mappings) > 0 {
			repo.SetStatusMap(sm.Mappings)
		}

		// Register pollers so serveCmd can keep manifests evergreen.
		// The closures capture repo before cache.New() wraps it, so Set*
		// reaches the live Repository directly.
		poller.Register("statuses:"+name, manifest.NewManifestPoller(
			"statuses",
			name,
			config.Dir(),
			manifest.DefaultTTL,
			func(ctx context.Context) (map[string]string, error) {
				entries, err := repo.ListStatuses(ctx)
				if err != nil {
					return nil, err
				}
				mappings := make(map[string]string, len(entries)+len(backend.Statuses))
				for _, e := range entries {
					mappings[e.Name] = defaultStatusMapping(e.CategoryKey)
				}
				maps.Copy(mappings, backend.Statuses)
				return mappings, nil
			},
			repo.SetStatusMap,
		))

		poller.Register("fields:"+name, manifest.NewManifestPoller(
			manifest.DefaultKind,
			name,
			config.Dir(),
			manifest.DefaultTTL,
			func(ctx context.Context) (map[string]string, error) {
				domainFields, err := repo.ListFields(ctx)
				if err != nil {
					return nil, err
				}
				mappings := make(map[string]string, len(domainFields))
				for _, f := range domainFields {
					if f.Custom {
						mappings[f.Name] = f.ID
					}
				}
				return mappings, nil
			},
			repo.SetCustomFields,
		))

		return repo, nil
	})
}
