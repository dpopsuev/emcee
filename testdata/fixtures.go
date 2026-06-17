// Package testdata provides pre-built Emcee domain fixtures for cross-service testing.
package testdata

import (
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
)

// SampleIssues returns a set of realistic issues spanning multiple backends.
func SampleIssues() []domain.Issue {
	return []domain.Issue{
		{
			Ref: "jira:AUTH-42", Key: "AUTH-42", Title: "Login fails with expired JWT",
			Description: "Users report 401 errors when JWT tokens expire during active sessions.",
			Status: domain.StatusInProgress, Priority: domain.PriorityHigh,
			Labels: []string{"auth", "security"}, Assignee: "alice",
			Project: "Auth Service", IssueType: "Bug",
			Components: []string{"token-validator", "session-manager"},
		},
		{
			Ref: "jira:AUTH-43", Key: "AUTH-43", Title: "Add refresh token rotation",
			Description: "Implement RFC 6749 refresh token rotation to prevent token reuse.",
			Status: domain.StatusTodo, Priority: domain.PriorityMedium,
			Labels: []string{"auth", "feature"}, Assignee: "bob",
			Project: "Auth Service", IssueType: "Story",
		},
		{
			Ref: "github:org/api#127", Key: "#127", Title: "API rate limiter returns 500 instead of 429",
			Description: "Under high load, the rate limiter panics and returns 500.",
			Status: domain.StatusTodo, Priority: domain.PriorityUrgent,
			Labels: []string{"api", "bug"}, Assignee: "charlie",
			Project: "API Gateway", IssueType: "Bug",
		},
	}
}

// SampleTriageGraph returns a triage result showing a defect lifecycle.
func SampleTriageGraph() *domain.TriageGraph {
	return &domain.TriageGraph{
		Seed: "jira:AUTH-42",
		Nodes: []domain.TriageNode{
			{Ref: "jira:AUTH-42", Type: "issue", Phase: "reported", Title: "Login fails with expired JWT", Status: "in_progress"},
			{Ref: "github:org/auth#88", Type: "pr", Phase: "fixed", Title: "Fix JWT expiry handling", Status: "merged"},
			{Ref: "jenkins:auth-ci/42", Type: "build", Phase: "verified", Title: "auth-ci #42", Status: "SUCCESS"},
		},
		Edges: []domain.TriageEdge{
			{From: "jira:AUTH-42", To: "github:org/auth#88", Type: "fixed_by", Confidence: 0.95, Source: "commit_msg"},
			{From: "github:org/auth#88", To: "jenkins:auth-ci/42", Type: "verified_by", Confidence: 1.0, Source: "ci_trigger"},
		},
	}
}

// SampleLaunch returns a Report Portal launch fixture.
func SampleLaunch() *domain.Launch {
	return &domain.Launch{
		ID: "37337", Name: "Regression Suite v3.2",
		Status: "FAILED", Description: "Nightly regression",
		StartTime: time.Date(2026, 6, 16, 2, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 16, 2, 45, 0, 0, time.UTC),
	}
}
