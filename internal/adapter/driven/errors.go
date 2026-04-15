// Package driven contains shared error types for driven (outbound) adapters.
package driven

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Rate limit constants.
const (
	// Default retry duration when Retry-After header is missing or invalid.
	defaultRetryAfter = 60 * time.Second

	// Common rate limit header names (GitHub/GitLab styles).
	headerRateLimitLimit     = "X-RateLimit-Limit"
	headerRateLimitRemaining = "X-RateLimit-Remaining"
	headerRateLimitReset     = "X-RateLimit-Reset"

	// GitLab alternative headers (without X- prefix).
	headerRateLimitLimitAlt     = "RateLimit-Limit"
	headerRateLimitRemainingAlt = "RateLimit-Remaining"
	headerRateLimitResetAlt     = "RateLimit-Reset"
)

// RateLimitError represents an API rate limit (HTTP 429) error.
// Contains information about when to retry and current quota status.
type RateLimitError struct {
	Backend    string        // Backend name (github, gitlab, jira, linear)
	RetryAfter time.Duration // Duration to wait before retrying
	Limit      int           // Total rate limit (0 if unknown)
	Remaining  int           // Requests remaining (0 if unknown)
	Reset      time.Time     // When the quota resets (zero if unknown)
	Message    string        // Original error message from API
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	msg := fmt.Sprintf("rate limit exceeded for %s", e.Backend)

	if e.RetryAfter > 0 {
		msg += fmt.Sprintf(", retry after %v", e.RetryAfter.Round(time.Second))
	}

	if e.Limit > 0 {
		msg += fmt.Sprintf(" (limit: %d", e.Limit)
		if e.Remaining >= 0 {
			msg += fmt.Sprintf(", remaining: %d", e.Remaining)
		}
		msg += ")"
	}

	if !e.Reset.IsZero() {
		msg += fmt.Sprintf(", resets at %s", e.Reset.Format(time.RFC3339))
	}

	if e.Message != "" {
		msg += fmt.Sprintf(": %s", e.Message)
	}

	return msg
}

// ParseRetryAfter parses the Retry-After header value.
// Supports both delay-seconds format (e.g., "60") and HTTP-date format (e.g., "Wed, 21 Oct 2026 07:28:00 GMT").
// Returns defaultRetryAfter if parsing fails.
func ParseRetryAfter(header string) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try parsing as seconds (most common)
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	// Try parsing as HTTP date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
	}

	// Fallback to conservative default
	return defaultRetryAfter
}

// ParseRateLimitHeaders extracts rate limit info from common API headers.
// Returns limit, remaining, and reset time (or zero values if not present).
func ParseRateLimitHeaders(headers http.Header) (limit, remaining int, reset time.Time) {
	// GitHub/GitLab style: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
	if limitStr := headers.Get(headerRateLimitLimit); limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	if remainingStr := headers.Get(headerRateLimitRemaining); remainingStr != "" {
		remaining, _ = strconv.Atoi(remainingStr)
	}
	if resetStr := headers.Get(headerRateLimitReset); resetStr != "" {
		if resetTimestamp, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			reset = time.Unix(resetTimestamp, 0)
		}
	}

	// GitLab also uses RateLimit-* (without X- prefix)
	if limit == 0 {
		if limitStr := headers.Get(headerRateLimitLimitAlt); limitStr != "" {
			limit, _ = strconv.Atoi(limitStr)
		}
	}
	if remaining == 0 {
		if remainingStr := headers.Get(headerRateLimitRemainingAlt); remainingStr != "" {
			remaining, _ = strconv.Atoi(remainingStr)
		}
	}
	if reset.IsZero() {
		if resetStr := headers.Get(headerRateLimitResetAlt); resetStr != "" {
			// GitLab uses ISO 8601 format
			if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
				reset = t
			}
		}
	}

	return limit, remaining, reset
}
