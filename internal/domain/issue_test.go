package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/dpopsuev/emcee/internal/domain"
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

func TestPriorityUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    domain.Priority
		wantErr bool
	}{
		{"int urgent", `1`, domain.PriorityUrgent, false},
		{"int high", `2`, domain.PriorityHigh, false},
		{"int none", `0`, domain.PriorityNone, false},
		{"string urgent", `"urgent"`, domain.PriorityUrgent, false},
		{"string high", `"high"`, domain.PriorityHigh, false},
		{"string medium", `"medium"`, domain.PriorityMedium, false},
		{"string low", `"low"`, domain.PriorityLow, false},
		{"string none", `"none"`, domain.PriorityNone, false},
		{"string unknown", `"garbage"`, domain.PriorityNone, false},
		{"invalid type", `true`, domain.PriorityNone, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p domain.Priority
			err := json.Unmarshal([]byte(tt.input), &p)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %s", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if p != tt.want {
				t.Errorf("got %v, want %v", p, tt.want)
			}
		})
	}
}

func TestPriorityJSONRoundTrip(t *testing.T) {
	type wrapper struct {
		Priority domain.Priority `json:"priority"`
	}

	// Marshal (should produce int)
	w := wrapper{Priority: domain.PriorityHigh}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"priority":2}` {
		t.Errorf("marshal = %s, want {\"priority\":2}", string(data))
	}

	// Unmarshal int
	var w2 wrapper
	if err := json.Unmarshal([]byte(`{"priority":2}`), &w2); err != nil {
		t.Fatalf("unmarshal int: %v", err)
	}
	if w2.Priority != domain.PriorityHigh {
		t.Errorf("unmarshal int = %v, want High", w2.Priority)
	}

	// Unmarshal string
	var w3 wrapper
	if err := json.Unmarshal([]byte(`{"priority":"high"}`), &w3); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if w3.Priority != domain.PriorityHigh {
		t.Errorf("unmarshal string = %v, want High", w3.Priority)
	}
}

func TestCreateInputBulkJSON(t *testing.T) {
	input := `[{"title":"Test","priority":"urgent"},{"title":"Test2","priority":3}]`
	var items []domain.CreateInput
	if err := json.Unmarshal([]byte(input), &items); err != nil {
		t.Fatalf("bulk unmarshal: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Priority != domain.PriorityUrgent {
		t.Errorf("item 0 priority = %v, want urgent", items[0].Priority)
	}
	if items[1].Priority != domain.PriorityMedium {
		t.Errorf("item 1 priority = %v, want medium", items[1].Priority)
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
