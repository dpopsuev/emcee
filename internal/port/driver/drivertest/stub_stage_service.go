package drivertest

import (
	"github.com/DanyPops/emcee/internal/app"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.StageService = (*StubStageService)(nil)

// StubStageService delegates to a real StageStore for functional testing.
// This is simpler than stubbing every method since StageStore is in-memory
// and has no external dependencies.
type StubStageService struct {
	store *app.StageStore
}

func NewStubStageService() *StubStageService {
	return &StubStageService{store: app.NewStageStore()}
}

func (s *StubStageService) StageItem(backend string, input domain.CreateInput, reason string) string {
	return s.store.StageItem(backend, input, reason)
}

func (s *StubStageService) StageList() []domain.StagedItem {
	return s.store.StageList()
}

func (s *StubStageService) StageGet(id string) (*domain.StagedItem, error) {
	return s.store.StageGet(id)
}

func (s *StubStageService) StagePatch(id string, input domain.UpdateInput) (*domain.StagedItem, error) {
	return s.store.StagePatch(id, input)
}

func (s *StubStageService) StageDrop(id string) error {
	return s.store.StageDrop(id)
}

func (s *StubStageService) StagePop(id string) (*domain.StagedItem, error) {
	return s.store.StagePop(id)
}

func (s *StubStageService) StagePopAll() []domain.StagedItem {
	return s.store.StagePopAll()
}
