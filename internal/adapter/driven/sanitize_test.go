package driven_test

import (
	"strings"
	"testing"

	"github.com/DanyPops/emcee/internal/adapter/driven"
)

func TestSanitizeError_TokenPatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    string
		notContains string
	}{
		{
			name:        "GitHub PAT in error",
			input:       "Authentication failed with token ghp_1234567890123456789012345678901234AB",
			contains:    "ghp_[REDACTED]",
			notContains: "ghp_123456789012345678901234567890123AB",
		},
		{
			name:        "GitLab PAT in error",
			input:       "Invalid token: glpat-abc123xyz456def7890123",
			contains:    "glpat-[REDACTED]",
			notContains: "glpat-abc123xyz456def7890123",
		},
		{
			name:        "GitLab file token",
			input:       "File token glft-Yi2n34pDii2XYabc1234567890 invalid",
			contains:    "glft-[REDACTED]",
			notContains: "glft-Yi2n34pDii2XYabc1234567890",
		},
		{
			name:        "Jira token",
			input:       "Token ATATT3xFfGN0abcxyz1234567890123456 rejected",
			contains:    "ATATT[REDACTED]",
			notContains: "ATATT3xFfGN0abcxyz1234567890123456",
		},
		{
			name:        "Linear API key",
			input:       "API key lin_api_NGg2PCqVQJhXRJMRSfWwk2pqrl2Lx7Tp rejected",
			contains:    "lin_api_[REDACTED]",
			notContains: "lin_api_NGg2PCqVQJhXRJMRSfWwk2pqrl2Lx7Tp",
		},
		{
			name:        "Bearer token with Linear key",
			input:       "Bearer lin_api_NGg2PCqVQJhXRJMRSfWwk2pqrl2Lx7Tp rejected",
			contains:    "[REDACTED]",
			notContains: "NGg2PCqVQJhXRJMRSfWwk2pqrl2Lx7Tp",
		},
		{
			name:        "Multiple tokens",
			input:       "Two tokens: glpat-abc123xyz456def7890123 and ghp_1234567890123456789012345678901234AB",
			contains:    "glpat-[REDACTED]",
			notContains: "glpat-abc123xyz456def7890123",
		},
		{
			name:        "No tokens",
			input:       "Resource not found",
			contains:    "Resource not found",
			notContains: "",
		},
		{
			name:        "Short token (no match)",
			input:       "Short glpat token",
			contains:    "glpat",
			notContains: "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := driven.SanitizeError(tt.input)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected output to contain %q, got: %s", tt.contains, result)
			}

			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("expected output to NOT contain %q, but it does: %s", tt.notContains, result)
			}
		})
	}
}
