package domain

import "time"

// LaunchView is the RP entry in the local materialized view.
// Identity Map only — RP items are read-only; defect_update is a direct write.
type LaunchView struct {
	Ref      string     `json:"ref"` // "reportportal:<id>"
	Launch   Launch     `json:"launch"`
	Items    []TestItem `json:"items"` // all items fetched at pull time
	PulledAt time.Time  `json:"pulled_at"`
}

// LaunchViewSummary is the lean form returned by view_list.
type LaunchViewSummary struct {
	Ref         string    `json:"ref"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	ItemsCached int       `json:"items_cached"`
	PulledAt    time.Time `json:"pulled_at"`
}

// Summary returns a lean view of the LaunchView for listing.
func (lv *LaunchView) Summary() LaunchViewSummary {
	return LaunchViewSummary{
		Ref:         lv.Ref,
		Name:        lv.Launch.Name,
		Status:      lv.Launch.Status,
		ItemsCached: len(lv.Items),
		PulledAt:    lv.PulledAt,
	}
}
