package driven_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/emcee/internal/adapter/driven"
)

func TestRateLimitError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *driven.RateLimitError
		contains []string
	}{
		{
			name: "with retry after",
			err: &driven.RateLimitError{
				Backend:    "github",
				RetryAfter: 60 * time.Second,
			},
			contains: []string{"rate limit exceeded", "github", "retry after 1m"},
		},
		{
			name: "with quota info",
			err: &driven.RateLimitError{
				Backend:   "gitlab",
				Limit:     5000,
				Remaining: 0,
			},
			contains: []string{"gitlab", "limit: 5000", "remaining: 0"},
		},
		{
			name: "with reset time",
			err: &driven.RateLimitError{
				Backend: "jira",
				Reset:   time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"jira", "resets at 2026-03-26T12:00:00Z"},
		},
		{
			name: "with message",
			err: &driven.RateLimitError{
				Backend: "linear",
				Message: "Too many requests",
			},
			contains: []string{"linear", "Too many requests"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("expected error to contain %q, got: %s", substr, errMsg)
				}
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{
			name:     "seconds format",
			header:   "60",
			expected: 60 * time.Second,
		},
		{
			name:     "seconds with whitespace",
			header:   "  120  ",
			expected: 120 * time.Second,
		},
		{
			name:     "HTTP date format",
			header:   "Wed, 21 Oct 2026 07:28:00 GMT",
			expected: time.Until(time.Date(2026, 10, 21, 7, 28, 0, 0, time.UTC)),
		},
		{
			name:     "empty header (default)",
			header:   "",
			expected: 60 * time.Second,
		},
		{
			name:     "invalid value (default)",
			header:   "invalid",
			expected: 60 * time.Second,
		},
		{
			name:     "negative seconds (default)",
			header:   "-5",
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := driven.ParseRetryAfter(tt.header)

			// For HTTP date, allow 1 second tolerance due to timing
			if tt.name == "HTTP date format" {
				diff := result - tt.expected
				if diff < 0 {
					diff = -diff
				}
				if diff > time.Second {
					t.Errorf("expected ~%v, got %v (diff: %v)", tt.expected, result, diff)
				}
			} else if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseRateLimitHeaders(t *testing.T) {
	tests := []struct {
		name              string
		headers           http.Header
		expectedLimit     int
		expectedRemaining int
		expectReset       bool
	}{
		{
			name: "GitHub style headers",
			headers: http.Header{
				"X-Ratelimit-Limit":     []string{"5000"},
				"X-Ratelimit-Remaining": []string{"4999"},
				"X-Ratelimit-Reset":     []string{"1711449600"}, // Unix timestamp
			},
			expectedLimit:     5000,
			expectedRemaining: 4999,
			expectReset:       true,
		},
		{
			name: "GitLab style headers (without X- prefix)",
			headers: http.Header{
				"Ratelimit-Limit":     []string{"2000"},
				"Ratelimit-Remaining": []string{"1500"},
				"Ratelimit-Reset":     []string{"2026-03-26T12:00:00Z"},
			},
			expectedLimit:     2000,
			expectedRemaining: 1500,
			expectReset:       true,
		},
		{
			name:              "no headers",
			headers:           http.Header{},
			expectedLimit:     0,
			expectedRemaining: 0,
			expectReset:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, remaining, reset := driven.ParseRateLimitHeaders(tt.headers)

			if limit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, limit)
			}
			if remaining != tt.expectedRemaining {
				t.Errorf("expected remaining %d, got %d", tt.expectedRemaining, remaining)
			}
			if tt.expectReset && reset.IsZero() {
				t.Error("expected reset time, got zero")
			}
			if !tt.expectReset && !reset.IsZero() {
				t.Errorf("expected no reset time, got %v", reset)
			}
		})
	}
}
