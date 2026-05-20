package stub

import (
	"context"

	"github.com/dpopsuev/emcee/internal/domain"
)

// StubViewService implements service.ViewService for testing.
type StubViewService struct {
	ViewRecords  map[string]*domain.ViewRecord
	ViewChanges  map[string]*domain.ChangeSet
	ViewPullFunc func(ctx context.Context, ref string) (any, error)
	PushFunc     func(ctx context.Context, ref string) (*domain.Issue, error)
}

func (s *StubViewService) ViewPull(ctx context.Context, ref string) (any, error) {
	if s.ViewPullFunc != nil {
		return s.ViewPullFunc(ctx, ref)
	}
	if s.ViewRecords != nil {
		if vr, ok := s.ViewRecords[ref]; ok {
			return vr, nil
		}
	}
	return nil, domain.ErrRecordNotFound
}

func (s *StubViewService) ViewGet(ref string) (any, error) {
	if s.ViewRecords != nil {
		if vr, ok := s.ViewRecords[ref]; ok {
			return vr, nil
		}
	}
	return nil, domain.ErrRecordNotFound
}

func (s *StubViewService) ViewMutate(ref, field, value string) error {
	if s.ViewRecords == nil {
		return domain.ErrRecordNotFound
	}
	vr, ok := s.ViewRecords[ref]
	if !ok {
		return domain.ErrRecordNotFound
	}
	old := vr.Fields[field]
	vr.Fields[field] = value
	if s.ViewChanges == nil {
		s.ViewChanges = make(map[string]*domain.ChangeSet)
	}
	cs, ok := s.ViewChanges[ref]
	if !ok {
		cs = &domain.ChangeSet{Ref: ref}
		s.ViewChanges[ref] = cs
	}
	cs.Changes = append(cs.Changes, domain.FieldChange{
		Field: field, OldValue: old, NewValue: value,
	})
	return nil
}

func (s *StubViewService) ViewDiff(ref string) (*domain.ViewDiff, error) {
	if s.ViewRecords == nil {
		return nil, domain.ErrRecordNotFound
	}
	if _, ok := s.ViewRecords[ref]; !ok {
		return nil, domain.ErrRecordNotFound
	}
	diff := &domain.ViewDiff{Ref: ref}
	if cs, ok := s.ViewChanges[ref]; ok {
		for _, c := range cs.Changes {
			diff.Changes = append(diff.Changes, domain.FieldDiff{
				Field: c.Field, LocalValue: c.NewValue, PullValue: c.OldValue,
			})
		}
	}
	return diff, nil
}

func (s *StubViewService) ViewPush(ctx context.Context, ref string) (*domain.Issue, error) {
	if s.PushFunc != nil {
		return s.PushFunc(ctx, ref)
	}
	return &domain.Issue{Ref: ref, Title: "pushed"}, nil
}

func (s *StubViewService) ViewList() any {
	result := make([]domain.ViewRecord, 0, len(s.ViewRecords))
	for _, vr := range s.ViewRecords {
		result = append(result, *vr)
	}
	return result
}

func (s *StubViewService) ViewDirty() []*domain.ChangeSet {
	result := make([]*domain.ChangeSet, 0, len(s.ViewChanges))
	for _, cs := range s.ViewChanges {
		result = append(result, cs)
	}
	return result
}

func (s *StubViewService) ViewPushAll(_ context.Context) ([]string, []string) {
	pushed := make([]string, 0, len(s.ViewChanges))
	for ref := range s.ViewChanges {
		pushed = append(pushed, ref)
	}
	s.ViewChanges = make(map[string]*domain.ChangeSet)
	return pushed, nil
}

func (s *StubViewService) ViewDrop(ref string) {
	delete(s.ViewRecords, ref)
	delete(s.ViewChanges, ref)
}

func (s *StubViewService) ViewReset() {
	s.ViewRecords = make(map[string]*domain.ViewRecord)
	s.ViewChanges = make(map[string]*domain.ChangeSet)
}
