package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	infra "github.com/dpopsuev/emcee/internal/infrastructure"
)

// CreateGist creates a new gist with the given filename and content.
func (r *Repository) CreateGist(ctx context.Context, filename, content string, public bool) (id, url string, err error) {
	if err := r.requireAuth(); err != nil {
		return "", "", err
	}
	infra.LogWrite(ctx, BackendName, "create_gist", slog.String(infra.LogKeyName, filename))
	start := time.Now()

	body := map[string]any{
		"public": public,
		"files": map[string]any{
			filename: map[string]string{"content": content},
		},
	}
	var result struct {
		ID      string `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	if err := r.api(ctx, http.MethodPost, "/gists", body, &result); err != nil {
		infra.LogError(ctx, BackendName, "create_gist", err)
		return "", "", err
	}
	infra.LogOpDone(ctx, BackendName, "create_gist", slog.Duration(infra.LogKeyElapsed, time.Since(start)))
	return result.ID, result.HTMLURL, nil
}

// UpdateGist updates an existing gist's file content.
func (r *Repository) UpdateGist(ctx context.Context, gistID, filename, content string) (string, error) {
	if err := r.requireAuth(); err != nil {
		return "", err
	}
	infra.LogWrite(ctx, BackendName, "update_gist", slog.String(infra.LogKeyID, gistID))
	start := time.Now()

	body := map[string]any{
		"files": map[string]any{
			filename: map[string]string{"content": content},
		},
	}
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	path := fmt.Sprintf("/gists/%s", gistID)
	if err := r.api(ctx, http.MethodPatch, path, body, &result); err != nil {
		infra.LogError(ctx, BackendName, "update_gist", err)
		return "", err
	}
	infra.LogOpDone(ctx, BackendName, "update_gist", slog.Duration(infra.LogKeyElapsed, time.Since(start)))
	return result.HTMLURL, nil
}
