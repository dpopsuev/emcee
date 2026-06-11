package timeexpr

import (
	"errors"
	"testing"
	"time"
)

// ref is a fixed Wednesday at 15:30 UTC used as the synthetic "now" for all tests.
// 2026-06-10 is a Wednesday; startOfWeek = Mon 2026-06-08, endOfWeek = Sun 2026-06-14.
var ref = time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)

// p is the test parser backed by FixedClock — deterministic, no real clock.
var p = New(FixedClock{T: ref})

func TestParse_RFC3339(t *testing.T) {
	got, err := p.Parse("2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParse_Empty(t *testing.T) {
	got, err := p.Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestParse_Whitespace(t *testing.T) {
	got, err := p.Parse("  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for whitespace-only input, got %v", got)
	}
}

func TestParse_Unknown(t *testing.T) {
	_, err := p.Parse("yesterday")
	if !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("expected ErrUnrecognized, got %v", err)
	}
}

// --- Named anchors (all resolved against ref = 2026-06-10 Wed 15:30 UTC) ---

func TestParse_Now(t *testing.T) {
	got, err := p.Parse("now")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(ref) {
		t.Errorf("now: got %v, want %v", got, ref)
	}
}

func TestParse_StartOfDay(t *testing.T) {
	got, err := p.Parse("startOfDay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("startOfDay: got %v, want %v", got, want)
	}
}

func TestParse_EndOfDay(t *testing.T) {
	got, err := p.Parse("endOfDay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 10, 23, 59, 59, 999999999, time.UTC)
	if !got.Equal(want) {
		t.Errorf("endOfDay: got %v, want %v", got, want)
	}
}

func TestParse_StartOfWeek(t *testing.T) {
	// ref is Wednesday 2026-06-10; ISO Monday is 2026-06-08.
	got, err := p.Parse("startOfWeek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("startOfWeek: got %v, want %v", got, want)
	}
}

func TestParse_StartOfWeek_Sunday(t *testing.T) {
	// Sunday edge case: 2026-06-14 is a Sunday; ISO Monday is 2026-06-08.
	sunday := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	ps := New(FixedClock{T: sunday})
	got, err := ps.Parse("startOfWeek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("startOfWeek(Sunday): got %v, want %v", got, want)
	}
}

func TestParse_EndOfWeek(t *testing.T) {
	// ref is Wednesday 2026-06-10; ISO Sunday is 2026-06-14 23:59:59.999...
	got, err := p.Parse("endOfWeek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 14, 23, 59, 59, 999999999, time.UTC)
	if !got.Equal(want) {
		t.Errorf("endOfWeek: got %v, want %v", got, want)
	}
}

func TestParse_StartOfMonth(t *testing.T) {
	got, err := p.Parse("startOfMonth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("startOfMonth: got %v, want %v", got, want)
	}
}

func TestParse_EndOfMonth(t *testing.T) {
	// June has 30 days.
	got, err := p.Parse("endOfMonth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 30, 23, 59, 59, 999999999, time.UTC)
	if !got.Equal(want) {
		t.Errorf("endOfMonth: got %v, want %v", got, want)
	}
}

func TestParse_AnchorCaseInsensitive(t *testing.T) {
	// Anchors must be case-insensitive: "STARTOFWEEK" == "startOfWeek".
	got, err := p.Parse("STARTOFWEEK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("STARTOFWEEK: got %v, want %v", got, want)
	}
}

// --- Relative offsets ---

func TestParse_RelativeDays(t *testing.T) {
	got, err := p.Parse("-7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ref.AddDate(0, 0, -7)
	if !got.Equal(want) {
		t.Errorf("-7d: got %v, want %v", got, want)
	}
}

func TestParse_RelativeWeeks(t *testing.T) {
	got, err := p.Parse("-2w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ref.AddDate(0, 0, -14)
	if !got.Equal(want) {
		t.Errorf("-2w: got %v, want %v", got, want)
	}
}

func TestParse_RelativeHours(t *testing.T) {
	got, err := p.Parse("-1h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ref.Add(-time.Hour)
	if !got.Equal(want) {
		t.Errorf("-1h: got %v, want %v", got, want)
	}
}

func TestParse_RelativeMinutes(t *testing.T) {
	got, err := p.Parse("-30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ref.Add(-30 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("-30m: got %v, want %v", got, want)
	}
}

func TestParse_RelativePositive(t *testing.T) {
	got, err := p.Parse("+1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ref.AddDate(0, 0, 1)
	if !got.Equal(want) {
		t.Errorf("+1d: got %v, want %v", got, want)
	}
}

// --- FixedClock satisfies Clock interface ---

func TestFixedClock(t *testing.T) {
	fixed := FixedClock{T: ref}
	if !fixed.Now().Equal(ref) {
		t.Errorf("FixedClock.Now() = %v, want %v", fixed.Now(), ref)
	}
}
