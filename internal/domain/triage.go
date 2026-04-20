package domain

import "time"

// TriageNode represents a single artifact in the defect lifecycle graph.
type TriageNode struct {
	Ref       string    `json:"ref"`   // backend:key (e.g. jira:OCPBUGS-123)
	Type      string    `json:"type"`  // issue, pr, build, launch, test_item
	Phase     string    `json:"phase"` // detected, stored, reported, fixed, verified
	Title     string    `json:"title"`
	URL       string    `json:"url,omitempty"`
	Status    string    `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// TriageEdge represents a link between two artifacts.
type TriageEdge struct {
	From       string  `json:"from"`       // source ref
	To         string  `json:"to"`         // target ref
	Type       string  `json:"type"`       // detected_by, stored_as, reported_in, fixed_by, verified_by, mentions
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Source     string  `json:"source"`     // where the link was found (description, comment, commit_msg)
}

// TriageGraph is the result of a triage query — a subgraph of the defect lifecycle.
type TriageGraph struct {
	Seed  string       `json:"seed"` // the ref that was triaged
	Nodes []TriageNode `json:"nodes"`
	Edges []TriageEdge `json:"edges"`
}

// CrossRef is a raw cross-reference extracted from text.
type CrossRef struct {
	Ref        string  `json:"ref"`    // backend:key
	Source     string  `json:"source"` // field where it was found
	Confidence float64 `json:"confidence"`
}
