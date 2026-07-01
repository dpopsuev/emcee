package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrRecordNotFound = errors.New("view record not found")
	ErrFieldNotFound  = errors.New("field not found in view record")
	ErrStaleView      = errors.New("remote record changed since pull")
)

// ViewRecord is one entity in the local materialized view — an issue pulled
// from a backend and stored as structured fields keyed by ref.
// Identity Map: exactly one ViewRecord per ref in the store.
type ViewRecord struct {
	Ref      string            `json:"ref"`
	Fields   map[string]string `json:"fields"`
	Version  string            `json:"version"`
	PulledAt time.Time         `json:"pulled_at"`
}

// FieldChange records a single field mutation.
type FieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// ChangeSet captures all dirty mutations on a single ViewRecord.
// Unit of Work: accumulated mutations flushed as one push.
type ChangeSet struct {
	Ref     string        `json:"ref"`
	Changes []FieldChange `json:"changes"`
}

// IsDirty reports whether the change set has any pending mutations.
func (cs *ChangeSet) IsDirty() bool {
	return len(cs.Changes) > 0
}

// DirtyFields returns the field names that have been mutated.
func (cs *ChangeSet) DirtyFields() []string {
	fields := make([]string, len(cs.Changes))
	for i, c := range cs.Changes {
		fields[i] = c.Field
	}
	return fields
}

// DirtyTracker manages change sets across multiple view records.
type DirtyTracker struct {
	sets map[string]*ChangeSet
}

// NewDirtyTracker creates a fresh tracker.
func NewDirtyTracker() *DirtyTracker {
	return &DirtyTracker{sets: make(map[string]*ChangeSet)}
}

// Mark records a field mutation for a ref.
func (dt *DirtyTracker) Mark(ref, field, oldValue, newValue string) {
	cs, ok := dt.sets[ref]
	if !ok {
		cs = &ChangeSet{Ref: ref}
		dt.sets[ref] = cs
	}
	cs.Changes = append(cs.Changes, FieldChange{
		Field:    field,
		OldValue: oldValue,
		NewValue: newValue,
	})
}

// Get returns the change set for a ref, or nil if clean.
func (dt *DirtyTracker) Get(ref string) *ChangeSet {
	return dt.sets[ref]
}

// All returns all dirty change sets.
func (dt *DirtyTracker) All() []*ChangeSet {
	result := make([]*ChangeSet, 0, len(dt.sets))
	for _, cs := range dt.sets {
		result = append(result, cs)
	}
	return result
}

// Clear removes the change set for a ref after successful push.
func (dt *DirtyTracker) Clear(ref string) {
	delete(dt.sets, ref)
}

// ClearAll resets all tracked changes.
func (dt *DirtyTracker) ClearAll() {
	dt.sets = make(map[string]*ChangeSet)
}

// IsDirty reports whether any ref has pending changes.
func (dt *DirtyTracker) IsDirty() bool {
	return len(dt.sets) > 0
}

// IssueToViewRecord converts a domain Issue to a ViewRecord with structured fields.
// All multi-value fields (labels, components, etc.) are stored as comma-separated strings.
func IssueToViewRecord(ref string, issue *Issue) ViewRecord {
	fields := map[string]string{
		"title":           issue.Title,
		"description":     issue.Description,
		"status":          string(issue.Status),
		"raw_status":      issue.RawStatus,
		"substatus":       issue.Substatus,
		"priority":        issue.Priority.String(),
		"assignee":        issue.Assignee,
		"reporter":        issue.Reporter,
		"url":             issue.URL,
		"sprint":          issue.Sprint,
		"labels":          strings.Join(issue.Labels, ","),
		"components":      strings.Join(issue.Components, ","),
		"fix_versions":    strings.Join(issue.FixVersions, ","),
		"target_versions": strings.Join(issue.TargetVersions, ","),
	}
	if issue.StoryPoints != nil {
		fields["story_points"] = fmt.Sprintf("%g", *issue.StoryPoints)
	}
	if issue.Parent != nil {
		fields["parent"] = issue.Parent.Key
	}
	// Merge arbitrary manifest-mapped custom fields.
	for k, v := range issue.CustomFields {
		fields[k] = v
	}
	return ViewRecord{
		Ref:      ref,
		Fields:   fields,
		Version:  issue.UpdatedAt.Format(time.RFC3339),
		PulledAt: time.Now(),
	}
}

// FieldValueFromIssue extracts a single field value from an Issue using the
// same mapping as IssueToViewRecord. Used by Push() for per-field conflict detection.
func FieldValueFromIssue(field string, issue *Issue) string {
	switch field {
	case "title":
		return issue.Title
	case "description":
		return issue.Description
	case "status":
		return string(issue.Status)
	case "raw_status":
		return issue.RawStatus
	case "substatus":
		return issue.Substatus
	case "priority":
		return issue.Priority.String()
	case "assignee":
		return issue.Assignee
	case "reporter":
		return issue.Reporter
	case "sprint":
		return issue.Sprint
	case "labels":
		return strings.Join(issue.Labels, ",")
	case "components":
		return strings.Join(issue.Components, ",")
	case "fix_versions":
		return strings.Join(issue.FixVersions, ",")
	case "target_versions":
		return strings.Join(issue.TargetVersions, ",")
	case "story_points":
		if issue.StoryPoints != nil {
			return fmt.Sprintf("%g", *issue.StoryPoints)
		}
		return ""
	case "parent":
		if issue.Parent != nil {
			return issue.Parent.Key
		}
		return ""
	}
	// Fallback: check arbitrary custom fields.
	if issue.CustomFields != nil {
		return issue.CustomFields[field]
	}
	return ""
}

// ViewDiff represents the difference between local and remote state.
type ViewDiff struct {
	Ref     string      `json:"ref"`
	Changes []FieldDiff `json:"changes"`
}

// FieldDiff represents the local vs remote state of a single field.
type FieldDiff struct {
	Field      string `json:"field"`
	LocalValue string `json:"local_value"`
	PullValue  string `json:"pull_value"`
}
