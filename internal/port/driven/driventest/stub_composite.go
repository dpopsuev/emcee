package driventest

// StubCompositeRepository implements all driven repository interfaces
// by embedding individual stubs. Use this when a test needs a repo
// that satisfies multiple interfaces (e.g., app.NewService).
type StubCompositeRepository struct {
	StubIssueRepository
	StubDocumentRepository
	StubProjectRepository
	StubInitiativeRepository
	StubLabelRepository
	StubBulkIssueRepository
	StubCommentRepository
	StubBuildRepository
}

// Name returns the name from the embedded StubIssueRepository,
// resolving the ambiguity from multiple embedded Name() methods.
func (c *StubCompositeRepository) Name() string {
	return c.StubIssueRepository.NameVal
}
