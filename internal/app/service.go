// Package app contains the application service — the hexagon's core orchestration layer.
// It implements the driver (inbound) port and delegates to driven (outbound) adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var (
	ErrUnknownBackend = errors.New("unknown backend")
	ErrInvalidRef     = errors.New("invalid ref")
)

// Service implements driver.IssueService by routing to the appropriate repository.
type Service struct {
	repos map[string]driven.IssueRepository
}

// NewService creates the application service with the given repositories.
func NewService(repos ...driven.IssueRepository) *Service {
	m := make(map[string]driven.IssueRepository, len(repos))
	for _, r := range repos {
		m[r.Name()] = r
	}
	return &Service{repos: m}
}

func (s *Service) repo(name string) (driven.IssueRepository, error) {
	r, ok := s.repos[name]
	if !ok {
		available := make([]string, 0, len(s.repos))
		for k := range s.repos {
			available = append(available, k)
		}
		return nil, fmt.Errorf("%w: %q (available: %s)", ErrUnknownBackend, name, strings.Join(available, ", "))
	}
	return r, nil
}

// ParseRef splits "linear:HEG-17" into backend and key.
func ParseRef(ref string) (backend, key string, err error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %q (expected backend:key, e.g. linear:HEG-17)", ErrInvalidRef, ref)
	}
	return parts[0], parts[1], nil
}

func (s *Service) List(ctx context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, ref string) (*domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, key)
}

func (s *Service) Create(ctx context.Context, backend string, input domain.CreateInput) (*domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Create(ctx, input)
}

func (s *Service) Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Update(ctx, key, input)
}

func (s *Service) Search(ctx context.Context, backend string, query string, limit int) ([]domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Search(ctx, query, limit)
}

func (s *Service) Backends() []string {
	names := make([]string, 0, len(s.repos))
	for k := range s.repos {
		names = append(names, k)
	}
	return names
}
