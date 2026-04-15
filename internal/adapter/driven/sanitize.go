// Package driven contains shared utilities for driven (outbound) adapters.
package driven

import (
	"regexp"
)

// Token pattern regex strings (for compile-time validation).
const (
	// GitHub Personal Access Token: ghp_ followed by 36 base62 chars.
	patternGitHubPAT = `ghp_[A-Za-z0-9]{36}` //nolint:gosec // G101: regex pattern, not a credential

	// GitLab Personal Access Token: glpat- followed by 20+ alphanumeric/dash/underscore.
	patternGitLabPAT = `glpat-[A-Za-z0-9_-]{20,}` //nolint:gosec // G101: regex pattern, not a credential

	// GitLab File Token: glft- followed by 20+ alphanumeric/dash/underscore.
	patternGitLabFileToken = `glft-[A-Za-z0-9_-]{20,}` //nolint:gosec // G101: regex pattern, not a credential

	// Jira Cloud API Token: ATATT followed by 20+ alphanumeric/dash/underscore.
	patternJiraToken = `ATATT[A-Za-z0-9_-]{20,}` //nolint:gosec // G101: regex pattern, not a credential

	// Linear API Key: lin_api_ followed by 32+ base62 chars.
	patternLinearAPIKey = `lin_api_[A-Za-z0-9]{32,}` //nolint:gosec // G101: regex pattern, not a credential

	// Generic Bearer token: Bearer followed by 20+ token chars.
	patternBearerToken = `Bearer\s+[A-Za-z0-9_-]{20,}` //nolint:gosec // G101: regex pattern, not a credential
)

// Redacted replacement strings (preserve token type prefix for debugging).
const (
	redactedGitHub = "ghp_[REDACTED]"
	redactedGitLab = "glpat-[REDACTED]"
	redactedGLFT   = "glft-[REDACTED]"
	redactedJira   = "ATATT[REDACTED]"
	redactedLinear = "lin_api_[REDACTED]"
	redactedBearer = "Bearer [REDACTED]" //nolint:gosec // G101: replacement string, not a credential
)

var (
	// Token patterns with replacement rules (compiled once for performance).
	tokenPatterns = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(patternGitHubPAT), redactedGitHub},
		{regexp.MustCompile(patternGitLabPAT), redactedGitLab},
		{regexp.MustCompile(patternGitLabFileToken), redactedGLFT},
		{regexp.MustCompile(patternJiraToken), redactedJira},
		{regexp.MustCompile(patternLinearAPIKey), redactedLinear},
		{regexp.MustCompile(patternBearerToken), redactedBearer},
	}
)

// SanitizeError removes sensitive API tokens from error messages.
// Prevents accidental token leakage in CLI output, logs, and MCP responses.
func SanitizeError(message string) string {
	result := message

	// Replace each token pattern with its redacted version
	for _, tp := range tokenPatterns {
		result = tp.pattern.ReplaceAllString(result, tp.replacement)
	}

	return result
}
