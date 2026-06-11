// Package timeexpr parses human-friendly time expressions used in since/before query parameters.
// Accepted forms:
//
//	RFC3339 timestamp     "2026-06-01T00:00:00Z"
//	Named anchor          now | startOfDay | endOfDay | startOfWeek | endOfWeek | startOfMonth | endOfMonth
//	Relative offset       -7d | -2w | -1h | -30m | +1d  (units: y M w d h m)
package timeexpr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	relativeRe = regexp.MustCompile(`^([+-]?\d+)(y|M|w|d|h|m)$`)

	// ErrUnrecognized is returned when the expression does not match any known form.
	ErrUnrecognized = errors.New("unrecognized time expression: want RFC3339, a named anchor (now, startOfWeek, endOfDay, …), or a relative offset (-7d, -2w, -1h)")
	// ErrUnknownUnit is returned when the relative-offset unit character is not recognised.
	ErrUnknownUnit = errors.New("unknown time unit")
)

// Parse resolves s to an absolute time.Time.
// It accepts RFC3339, named anchors, and relative offsets.
// An empty string returns the zero time without error.
func Parse(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	now := time.Now()
	switch strings.ToLower(s) {
	case "now":
		return now, nil
	case "startofday":
		return startOfDay(now), nil
	case "endofday":
		return endOfDay(now), nil
	case "startofweek":
		return startOfWeek(now), nil
	case "endofweek":
		return endOfWeek(now), nil
	case "startofmonth":
		return startOfMonth(now), nil
	case "endofmonth":
		return endOfMonth(now), nil
	}

	m := relativeRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, fmt.Errorf("%q: %w", s, ErrUnrecognized)
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "y":
		return now.AddDate(n, 0, 0), nil
	case "M":
		return now.AddDate(0, n, 0), nil
	case "w":
		return now.AddDate(0, 0, n*7), nil
	case "d":
		return now.AddDate(0, 0, n), nil
	case "h":
		return now.Add(time.Duration(n) * time.Hour), nil
	case "m":
		return now.Add(time.Duration(n) * time.Minute), nil
	}
	return time.Time{}, fmt.Errorf("%q: %w", m[2], ErrUnknownUnit)
}

func startOfDay(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 23, 59, 59, 999999999, t.Location())
}

// startOfWeek returns the Monday 00:00 of t's ISO week.
func startOfWeek(t time.Time) time.Time {
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	return startOfDay(t.AddDate(0, 0, -int(wd-time.Monday)))
}

func endOfWeek(t time.Time) time.Time {
	return endOfDay(startOfWeek(t).AddDate(0, 0, 6))
}

func startOfMonth(t time.Time) time.Time {
	y, mo, _ := t.Date()
	return time.Date(y, mo, 1, 0, 0, 0, 0, t.Location())
}

func endOfMonth(t time.Time) time.Time {
	return startOfMonth(t).AddDate(0, 1, 0).Add(-time.Nanosecond)
}
