package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
)

const (
	viewLogMsg       = "view"
	viewLogKeyOp     = "op"
	viewLogKeyRef    = "ref"
	viewLogKeyFields = "dirty_fields"
)

// ViewStore is the in-memory local materialized view.
// Identity Map: one ViewRecord per ref, no duplicate fetches.
// Unit of Work: DirtyTracker accumulates mutations, push flushes them.
type ViewStore struct {
	records map[string]*domain.ViewRecord
	tracker *domain.DirtyTracker
	mu      sync.RWMutex
}

// NewViewStore creates an empty view store.
func NewViewStore() *ViewStore {
	return &ViewStore{
		records: make(map[string]*domain.ViewRecord),
		tracker: domain.NewDirtyTracker(),
	}
}

// Pull fetches an issue from the backend and stores it as a ViewRecord.
func (vs *ViewStore) Pull(ctx context.Context, svc *Service, ref string) (*domain.ViewRecord, error) {
	start := time.Now()
	issue, err := svc.Get(ctx, ref)
	if err != nil {
		return nil, err
	}

	vr := domain.IssueToViewRecord(ref, issue)

	vs.mu.Lock()
	vs.records[ref] = &vr
	vs.tracker.Clear(ref)
	vs.mu.Unlock()

	slog.LogAttrs(ctx, slog.LevelDebug, viewLogMsg,
		slog.String(viewLogKeyOp, "pull"),
		slog.String(viewLogKeyRef, ref),
		slog.Duration("elapsed", time.Since(start)),
	)
	return &vr, nil
}

// Get returns a ViewRecord from the local store without hitting the backend.
func (vs *ViewStore) Get(ref string) (*domain.ViewRecord, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	vr, ok := vs.records[ref]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	cp := *vr
	cp.Fields = make(map[string]string, len(vr.Fields))
	maps.Copy(cp.Fields, vr.Fields)
	return &cp, nil
}

// Mutate changes a single field on a local ViewRecord and marks it dirty.
func (vs *ViewStore) Mutate(ref, field, value string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vr, ok := vs.records[ref]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	old := vr.Fields[field]
	if old == value {
		return nil
	}
	vr.Fields[field] = value
	vs.tracker.Mark(ref, field, old, value)
	slog.LogAttrs(context.Background(), slog.LevelDebug, viewLogMsg,
		slog.String(viewLogKeyOp, "mutate"),
		slog.String(viewLogKeyRef, ref),
		slog.String("field", field),
	)
	return nil
}

// Diff returns the changes made to a local ViewRecord since it was pulled.
func (vs *ViewStore) Diff(ref string) (*domain.ViewDiff, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	vr, ok := vs.records[ref]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	cs := vs.tracker.Get(ref)
	if cs == nil {
		return &domain.ViewDiff{Ref: ref}, nil
	}
	diff := &domain.ViewDiff{Ref: ref}
	seen := make(map[string]bool)
	for _, c := range cs.Changes {
		if seen[c.Field] {
			continue
		}
		seen[c.Field] = true
		diff.Changes = append(diff.Changes, domain.FieldDiff{
			Field:      c.Field,
			LocalValue: vr.Fields[c.Field],
			PullValue:  c.OldValue,
		})
	}
	return diff, nil
}

// Push flushes dirty fields for a ref back to the backend via Update.
func (vs *ViewStore) Push(ctx context.Context, svc *Service, ref string) (*domain.Issue, error) {
	start := time.Now()
	vs.mu.RLock()
	vr, ok := vs.records[ref]
	cs := vs.tracker.Get(ref)
	vs.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	if cs == nil || !cs.IsDirty() {
		return nil, nil
	}

	remote, fetchErr := svc.Get(ctx, ref)
	if fetchErr != nil {
		return nil, fetchErr
	}
	// Field-level conflict detection: only reject if a dirty field was also
	// changed remotely since we pulled. Concurrent edits to different fields are allowed.
	if conflict := fieldConflict(cs, remote); conflict != "" {
		return nil, fmt.Errorf("%w: field %q changed remotely since pull", domain.ErrStaleView, conflict)
	}

	input := vs.buildUpdateInput(vr, cs)
	dirtyCount := len(cs.Changes)

	issue, err := svc.Update(ctx, ref, input)
	if err != nil {
		return nil, err
	}

	vs.mu.Lock()
	updated := domain.IssueToViewRecord(ref, issue)
	vs.records[ref] = &updated
	vs.tracker.Clear(ref)
	vs.mu.Unlock()

	slog.LogAttrs(ctx, slog.LevelDebug, viewLogMsg,
		slog.String(viewLogKeyOp, "push"),
		slog.String(viewLogKeyRef, ref),
		slog.Int(viewLogKeyFields, dirtyCount),
		slog.Duration("elapsed", time.Since(start)),
	)
	return issue, nil
}

// buildUpdateInput constructs an UpdateInput from the current view state and dirty fields.
func (vs *ViewStore) buildUpdateInput(vr *domain.ViewRecord, cs *domain.ChangeSet) domain.UpdateInput {
	dirty := make(map[string]bool)
	for _, c := range cs.Changes {
		dirty[c.Field] = true
	}

	var input domain.UpdateInput
	if dirty["title"] {
		v := vr.Fields["title"]
		input.Title = &v
	}
	if dirty["description"] {
		v := vr.Fields["description"]
		input.Description = &v
	}
	if dirty["status"] {
		s := domain.Status(vr.Fields["status"])
		input.Status = &s
	}
	if dirty["priority"] {
		p := domain.ParsePriority(vr.Fields["priority"])
		input.Priority = &p
	}
	if dirty["assignee"] {
		v := vr.Fields["assignee"]
		input.Assignee = &v
	}
	if dirty["labels"] {
		input.Labels = splitCSV(vr.Fields["labels"])
	}
	if dirty["components"] {
		input.Components = splitCSV(vr.Fields["components"])
	}
	if dirty["fix_versions"] {
		input.FixVersions = splitCSV(vr.Fields["fix_versions"])
	}
	return input
}

// fieldConflict returns the name of the first dirty field that was also changed
// on the remote since the record was pulled, or "" if there is no conflict.
func fieldConflict(cs *domain.ChangeSet, remote *domain.Issue) string {
	for _, c := range cs.Changes {
		if domain.FieldValueFromIssue(c.Field, remote) != c.OldValue {
			return c.Field
		}
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// PushAll flushes all dirty records to their backends.
// Returns the list of refs that were successfully pushed and any errors.
func (vs *ViewStore) PushAll(ctx context.Context, svc *Service) (pushed []string, errs []string) {
	vs.mu.RLock()
	dirty := vs.tracker.All()
	vs.mu.RUnlock()

	for _, cs := range dirty {
		issue, err := vs.Push(ctx, svc, cs.Ref)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", cs.Ref, err))
			continue
		}
		if issue != nil {
			pushed = append(pushed, cs.Ref)
		}
	}
	return pushed, errs
}

// List returns all ViewRecords in the store.
func (vs *ViewStore) List() []domain.ViewRecord {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	result := make([]domain.ViewRecord, 0, len(vs.records))
	for _, vr := range vs.records {
		result = append(result, *vr)
	}
	return result
}

// Dirty returns all pending change sets.
func (vs *ViewStore) Dirty() []*domain.ChangeSet {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.tracker.All()
}

// Drop removes a single ref from the view and its change set.
func (vs *ViewStore) Drop(ref string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.records, ref)
	vs.tracker.Clear(ref)
}

// Reset clears all records and change sets.
func (vs *ViewStore) Reset() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.records = make(map[string]*domain.ViewRecord)
	vs.tracker.ClearAll()
}
