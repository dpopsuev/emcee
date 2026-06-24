package stub

import "github.com/dpopsuev/emcee/internal/service"

var _ service.ProjectScopeService = (*StubProjectScopeService)(nil)

type StubProjectScopeService struct {
	Project string
}

func (s *StubProjectScopeService) DefaultProject(_ string) string { return s.Project }

func (s *StubProjectScopeService) SetDefaultProject(_, project string) error {
	s.Project = project
	return nil
}
