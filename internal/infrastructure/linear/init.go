package linear

import (
	"os"

	"github.com/dpopsuev/emcee/internal/config"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

func init() {
	infra.Register("linear", 0, func(name string, backend config.Backend) (repository.IssueRepository, error) {
		key := backend.ResolveKey()
		if key == "" {
			key = os.Getenv("LINEAR_API_KEY")
		}
		if key == "" {
			return nil, nil
		}
		team := backend.Team
		if team == "" {
			team = os.Getenv("LINEAR_TEAM")
		}
		if team == "" {
			team = "HEG"
		}
		return New(name, key, team)
	})
}
