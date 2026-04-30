package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	adapterdriven "github.com/dpopsuev/emcee/internal/adapter/driven"
)

// CreateGist creates a new gist with the given filename and content.
func (r *Repository) CreateGist(ctx context.Context, filename, content string, public bool) (id, url string, err error) {
	if err := r.requireAuth(); err != nil {
		return "", "", err
	}
	adapterdriven.LogWrite(ctx, BackendName, "create_gist", slog.String(adapterdriven.LogKeyName, filename))
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
		adapterdriven.LogError(ctx, BackendName, "create_gist", err)
		return "", "", err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "create_gist", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return result.ID, result.HTMLURL, nil
}

// UpdateGist updates an existing gist's file content.
func (r *Repository) UpdateGist(ctx context.Context, gistID, filename, content string) (string, error) {
	if err := r.requireAuth(); err != nil {
		return "", err
	}
	adapterdriven.LogWrite(ctx, BackendName, "update_gist", slog.String(adapterdriven.LogKeyID, gistID))
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
		adapterdriven.LogError(ctx, BackendName, "update_gist", err)
		return "", err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "update_gist", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return result.HTMLURL, nil
}
