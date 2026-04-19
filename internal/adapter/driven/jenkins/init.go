package jenkins

import (
	"context"
	"os"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/port/driven"
)

func init() {
	adapterdriven.Register("jenkins", 0, func(name string, backend config.Backend) (driven.IssueRepository, error) {
		token := backend.ResolveKey()
		if token == "" {
			token = os.Getenv("JENKINS_API_KEY")
		}
		if token == "" {
			return nil, nil
		}
		url := backend.URL
		if url == "" {
			url = os.Getenv("JENKINS_URL")
		}
		if url == "" {
			return nil, nil
		}
		user := backend.Email
		if user == "" {
			user = os.Getenv("JENKINS_USER")
		}
		if user == "" {
			return nil, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return New(ctx, url, user, token)
	})
}
