package timeexpr

import (
	"errors"
	"testing"
	"time"
)

func TestParse_RFC3339(t *testing.T) {
	got, err := Parse("2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParse_Empty(t *testing.T) {
	got, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestParse_Unknown(t *testing.T) {
	_, err := Parse("yesterday")
	if !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("expected ErrUnrecognized, got %v", err)
	}
}

func TestParse_RelativeOffsets(t *testing.T) {
	cases := []struct {
		expr string
		unit time.Duration
		days int
	}{
		{"-1h", -time.Hour, 0},
		{"+2h", 2 * time.Hour, 0},
		{"-30m", -30 * time.Minute, 0},
	}
	for _, tc := range cases {
		before := time.Now()
		got, err := Parse(tc.expr)
		after := time.Now()
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.expr, err)
			continue
		}
		// Got must be within [before+unit-1s, after+unit+1s].
		lo := before.Add(tc.unit - time.Second)
		hi := after.Add(tc.unit + time.Second)
		if got.Before(lo) || got.After(hi) {
			t.Errorf("Parse(%q) = %v, want in [%v, %v]", tc.expr, got, lo, hi)
		}
	}
}

func TestParse_RelativeDays(t *testing.T) {
	got, err := Parse("-7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Now().AddDate(0, 0, -7)
	diff := got.Sub(want)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Parse(-7d) off by %v", diff)
	}
}

func TestParse_RelativeWeeks(t *testing.T) {
	got, err := Parse("-2w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Now().AddDate(0, 0, -14)
	diff := got.Sub(want)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Parse(-2w) off by %v", diff)
	}
}

func TestParse_StartOfWeek(t *testing.T) {
	// Use a known Wednesday: 2026-06-10 (Wednesday).
	ref := time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)
	sow := startOfWeek(ref)
	// Monday of that week is 2026-06-08.
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if !sow.Equal(want) {
		t.Errorf("startOfWeek(Wed 2026-06-10) = %v, want %v", sow, want)
	}
}

func TestParse_EndOfWeek(t *testing.T) {
	ref := time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)
	eow := endOfWeek(ref)
	// Sunday of that week is 2026-06-14 23:59:59.999...
	want := time.Date(2026, 6, 14, 23, 59, 59, 999999999, time.UTC)
	if !eow.Equal(want) {
		t.Errorf("endOfWeek(Wed 2026-06-10) = %v, want %v", eow, want)
	}
}

func TestParse_StartOfMonth(t *testing.T) {
	ref := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	som := startOfMonth(ref)
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !som.Equal(want) {
		t.Errorf("startOfMonth = %v, want %v", som, want)
	}
}

func TestParse_EndOfMonth(t *testing.T) {
	ref := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	eom := endOfMonth(ref)
	// June has 30 days
	want := time.Date(2026, 6, 30, 23, 59, 59, 999999999, time.UTC)
	if !eom.Equal(want) {
		t.Errorf("endOfMonth = %v, want %v", eom, want)
	}
}

func TestParse_NamedAnchors(t *testing.T) {
	anchors := []string{"now", "startOfDay", "endOfDay", "startOfWeek", "endOfWeek", "startOfMonth", "endOfMonth"}
	for _, a := range anchors {
		got, err := Parse(a)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", a, err)
			continue
		}
		if got.IsZero() {
			t.Errorf("Parse(%q) returned zero time", a)
		}
	}
}
