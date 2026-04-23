package drivertest

import (
	"sync"

	"github.com/dpopsuev/emcee/internal/port/driven"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.BackendManager = (*StubBackendManager)(nil)

type AddBackendCall struct {
	Name string
}

type RemoveBackendCall struct {
	Name string
}

type ReloadConfigCall struct {
	Path string
}

type StubBackendManager struct {
	RemoveResult  bool
	ReloadAdded   []string
	ReloadRemoved []string
	ReloadErr     error
	Err           error

	mu                 sync.Mutex
	AddBackendCalls    []AddBackendCall
	RemoveBackendCalls []RemoveBackendCall
	ReloadConfigCalls  []ReloadConfigCall
}

func (s *StubBackendManager) AddBackend(repo driven.IssueRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AddBackendCalls = append(s.AddBackendCalls, AddBackendCall{Name: repo.Name()})
}

func (s *StubBackendManager) RemoveBackend(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RemoveBackendCalls = append(s.RemoveBackendCalls, RemoveBackendCall{Name: name})
	return s.RemoveResult
}

func (s *StubBackendManager) ReloadConfig(configPath string) (added, removed []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReloadConfigCalls = append(s.ReloadConfigCalls, ReloadConfigCall{Path: configPath})
	return s.ReloadAdded, s.ReloadRemoved, s.ReloadErr
}
