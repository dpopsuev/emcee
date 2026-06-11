package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/repository"
)

// ErrStaleView is returned by Get when a non-terminal launch has not been
// re-pulled within staleTTL. Callers should re-pull to get fresh data.
var ErrStaleView = errors.New("launch view is stale")

// staleTTL is the maximum age of a cached launch view for a non-terminal launch.
// Terminal launches (PASSED, FAILED, STOPPED) are immutable and never stale.
const staleTTL = 5 * time.Minute

// terminalStatus reports whether a launch status can no longer change.
func terminalStatus(status string) bool {
	switch status {
	case "PASSED", "FAILED", "STOPPED":
		return true
	}
	return false
}

// Package-level sentinel for missing launch backend — reuses ErrNotSupported from service.go.
// Declared here to avoid import cycles; the actual ErrNotSupported is in service.go.

// LaunchViewStore is the in-memory local materialized view for Report Portal.
// Identity Map only: one LaunchView per ref, no duplicate fetches.
// No Unit of Work — RP items are read-only; defect_update bypasses the view.
type LaunchViewStore struct {
	records map[string]*domain.LaunchView
	mu      sync.RWMutex
}

func newLaunchViewStore() *LaunchViewStore {
	return &LaunchViewStore{records: make(map[string]*domain.LaunchView)}
}

// Pull fetches a launch and all its test items from the backend and caches them.
// ref must be in "backend:id" form, e.g. "reportportal:37337".
func (ls *LaunchViewStore) Pull(ctx context.Context, backend, id string, repos map[string]repository.LaunchRepository) (*domain.LaunchView, error) {
	ref := backend + ":" + id

	lr, ok := repos[backend]
	if !ok {
		return nil, fmt.Errorf("%w: %q does not support launches", ErrNotSupported, backend)
	}

	start := time.Now()

	launch, err := lr.GetLaunch(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get launch: %w", err)
	}

	items, err := lr.ListTestItems(ctx, id, domain.TestItemFilter{Limit: 500, IncludeLogs: true})
	if err != nil {
		return nil, fmt.Errorf("list test items: %w", err)
	}

	lv := &domain.LaunchView{
		Ref:      ref,
		Launch:   *launch,
		Items:    items,
		PulledAt: time.Now(),
	}

	ls.mu.Lock()
	ls.records[ref] = lv
	ls.mu.Unlock()

	slog.LogAttrs(ctx, slog.LevelDebug, viewLogMsg,
		slog.String(viewLogKeyOp, "launch_pull"),
		slog.String(viewLogKeyRef, ref),
		slog.Int("items", len(items)),
		slog.Duration("elapsed", time.Since(start)),
	)
	return lv, nil
}

// Get returns a cached LaunchView. Returns ErrRecordNotFound if not pulled.
// Returns ErrStaleView (alongside the stale view) when a non-terminal launch
// has not been refreshed within staleTTL — the caller should re-pull.
func (ls *LaunchViewStore) Get(ref string) (*domain.LaunchView, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	lv, ok := ls.records[ref]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	if !terminalStatus(lv.Launch.Status) && time.Since(lv.PulledAt) > staleTTL {
		return lv, ErrStaleView
	}
	return lv, nil
}

// GetItems returns cached test items for a launch, with optional status filter.
// Returns nil, false if the launch is not in the cache.
// An empty statuses slice returns all items.
func (ls *LaunchViewStore) GetItems(ref string, statuses []string) ([]domain.TestItem, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	lv, ok := ls.records[ref]
	if !ok {
		return nil, false
	}
	if len(statuses) == 0 {
		return lv.Items, true
	}
	allowed := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		allowed[strings.ToUpper(s)] = true
	}
	var out []domain.TestItem
	for _, it := range lv.Items {
		if allowed[strings.ToUpper(it.Status)] {
			out = append(out, it)
		}
	}
	return out, true
}

// List returns lean summaries of all cached launches.
func (ls *LaunchViewStore) List() []domain.LaunchViewSummary {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	result := make([]domain.LaunchViewSummary, 0, len(ls.records))
	for _, lv := range ls.records {
		result = append(result, lv.Summary())
	}
	return result
}

// Drop evicts a launch from the cache.
func (ls *LaunchViewStore) Drop(ref string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	delete(ls.records, ref)
}

// Reset clears all cached launches.
func (ls *LaunchViewStore) Reset() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.records = make(map[string]*domain.LaunchView)
}

// BuildTree returns the launch item hierarchy rooted at the top-level suites.
// The launch must have been pulled first; returns ErrRecordNotFound otherwise.
func (ls *LaunchViewStore) BuildTree(ref string) ([]*domain.ItemTreeNode, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	lv, ok := ls.records[ref]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrRecordNotFound, ref)
	}
	return buildItemTree(lv.Items), nil
}

// buildItemTree constructs a tree from a flat item list using ParentID references.
// Items whose ParentID is absent from the list (including "0" and "") are roots.
func buildItemTree(items []domain.TestItem) []*domain.ItemTreeNode {
	nodes := make(map[string]*domain.ItemTreeNode, len(items))
	for i := range items {
		it := &items[i]
		nodes[it.ID] = &domain.ItemTreeNode{
			ID:        it.ID,
			Name:      it.Name,
			Status:    it.Status,
			Type:      it.Type,
			IssueType: it.IssueType,
		}
	}
	var roots []*domain.ItemTreeNode
	for i := range items {
		it := &items[i]
		node := nodes[it.ID]
		if parent, ok := nodes[it.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}
