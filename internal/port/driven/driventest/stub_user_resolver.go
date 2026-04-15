package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.UserResolver = (*StubUserResolver)(nil)

type ResolveUserCall struct {
	Name string
}

type StubUserResolver struct {
	UserID string
	Err    error

	mu    sync.Mutex
	Calls []ResolveUserCall
}

func (s *StubUserResolver) ResolveUser(_ context.Context, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, ResolveUserCall{Name: name})
	return s.UserID, s.Err
}
