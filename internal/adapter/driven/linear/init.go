package linear

import (
	"os"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

func init() {
	adapterdriven.Register("linear", 0, func(name string, backend config.Backend) (driven.IssueRepository, error) {
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
