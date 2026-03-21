package domain_test

import (
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input string
		want  domain.Priority
	}{
		{"urgent", domain.PriorityUrgent},
		{"high", domain.PriorityHigh},
		{"medium", domain.PriorityMedium},
		{"low", domain.PriorityLow},
		{"none", domain.PriorityNone},
		{"", domain.PriorityNone},
		{"URGENT", domain.PriorityNone}, // case-sensitive
		{"garbage", domain.PriorityNone},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := domain.ParsePriority(tt.input)
			if got != tt.want {
				t.Errorf("ParsePriority(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriorityString(t *testing.T) {
	tests := []struct {
		input domain.Priority
		want  string
	}{
		{domain.PriorityUrgent, "urgent"},
		{domain.PriorityHigh, "high"},
		{domain.PriorityMedium, "medium"},
		{domain.PriorityLow, "low"},
		{domain.PriorityNone, "none"},
		{domain.Priority(99), "none"}, // unknown value
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.input.String()
			if got != tt.want {
				t.Errorf("Priority(%d).String() = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriorityRoundTrip(t *testing.T) {
	for _, p := range []domain.Priority{
		domain.PriorityUrgent, domain.PriorityHigh,
		domain.PriorityMedium, domain.PriorityLow,
	} {
		got := domain.ParsePriority(p.String())
		if got != p {
			t.Errorf("roundtrip: %v -> %q -> %v", p, p.String(), got)
		}
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []domain.Status{
		domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress,
		domain.StatusInReview, domain.StatusDone, domain.StatusCanceled,
	}
	seen := make(map[domain.Status]bool)
	for _, s := range statuses {
		if s == "" {
			t.Errorf("status constant is empty")
		}
		if seen[s] {
			t.Errorf("duplicate status: %q", s)
		}
		seen[s] = true
	}
}
