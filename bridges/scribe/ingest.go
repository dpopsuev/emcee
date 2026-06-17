package scribe

import (
	"context"

	scribeclient "github.com/dpopsuev/scribe/client"
	"github.com/dpopsuev/emcee/internal/domain"
)

// IngestIssues translates issues and POSTs to the Scribe ingest URL.
func IngestIssues(ctx context.Context, issues []domain.Issue, ingestURL string) error {
	result := TranslateIssues(issues)
	return scribeclient.Post(ctx, result.Records, result.Edges, "emcee", ingestURL)
}

// IngestTriageGraph translates a triage graph and POSTs to the Scribe ingest URL.
func IngestTriageGraph(ctx context.Context, graph *domain.TriageGraph, ingestURL string) error {
	result := TranslateTriageGraph(graph)
	return scribeclient.Post(ctx, result.Records, result.Edges, "emcee", ingestURL)
}
