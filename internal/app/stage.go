package app

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
)

// ErrStageItemNotFound indicates a stage item was not found.
var ErrStageItemNotFound = errors.New("stage item not found")

const (
	stageTTL   = 30 * time.Minute
	stageLimit = 50
)

// StageStore is an in-memory pre-submission cache for issue payloads.
// Items are staged locally and pushed to backends when ready.
type StageStore struct {
	mu    sync.Mutex
	items map[string]*domain.StagedItem
	now   func() time.Time
}

// NewStageStore creates a new stage store.
func NewStageStore() *StageStore {
	return &StageStore{
		items: make(map[string]*domain.StagedItem),
		now:   time.Now,
	}
}

// StageItem adds an issue payload to the staging cache. Returns the stage ID.
func (s *StageStore) StageItem(backend string, input domain.CreateInput, reason string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	id := generateID()
	s.items[id] = &domain.StagedItem{
		ID:        id,
		Backend:   backend,
		Input:     input,
		Reason:    reason,
		CreatedAt: s.now(),
	}
	return id
}

// StageList returns all staged items.
func (s *StageStore) StageList() []domain.StagedItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	out := make([]domain.StagedItem, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, *item)
	}
	return out
}

// StageGet returns a staged item by ID.
func (s *StageStore) StageGet(id string) (*domain.StagedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrStageItemNotFound, id)
	}
	return item, nil
}

// StagePatch updates fields on a staged item. Non-empty/non-nil fields override.
func (s *StageStore) StagePatch(id string, input domain.UpdateInput) (*domain.StagedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrStageItemNotFound, id)
	}
	if input.Title != nil {
		item.Input.Title = *input.Title
	}
	if input.Description != nil {
		item.Input.Description = *input.Description
	}
	if input.Status != nil {
		item.Input.Status = *input.Status
	}
	if input.Priority != nil {
		item.Input.Priority = *input.Priority
	}
	if input.Assignee != nil {
		item.Input.Assignee = *input.Assignee
	}
	if input.Labels != nil {
		item.Input.Labels = input.Labels
	}
	return item, nil
}

// StageDrop removes a staged item.
func (s *StageStore) StageDrop(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("%w: %q", ErrStageItemNotFound, id)
	}
	delete(s.items, id)
	return nil
}

// StagePop removes and returns a staged item for pushing.
func (s *StageStore) StagePop(id string) (*domain.StagedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrStageItemNotFound, id)
	}
	delete(s.items, id)
	return item, nil
}

// StagePopAll removes and returns all staged items.
func (s *StageStore) StagePopAll() []domain.StagedItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	out := make([]domain.StagedItem, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, *item)
	}
	s.items = make(map[string]*domain.StagedItem)
	return out
}

func (s *StageStore) evictExpired() {
	cutoff := s.now().Add(-stageTTL)
	for id, item := range s.items {
		if item.CreatedAt.Before(cutoff) {
			delete(s.items, id)
		}
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
