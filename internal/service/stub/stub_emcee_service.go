package stub

import (
	mcpdriver "github.com/dpopsuev/emcee/internal/api/mcp"
)

var _ mcpdriver.EmceeService = (*StubEmceeService)(nil)

// StubEmceeService implements mcpdriver.EmceeService by embedding
// all driver port stubs. Use this for MCP server testing.
type StubEmceeService struct {
	StubIssueService
	StubDocumentService
	StubProjectService
	StubInitiativeService
	StubLabelService
	StubBulkService
	StubHealthService
	StubCommentService
	StubStageService
	StubChangelogService
	StubFieldService
	StubTemplateService
	StubJQLService
	StubPRService
	StubLaunchService
	StubGistService
	StubPRReviewService
	StubIssueLinkService
	StubBackendManager
	StubTriageService
	StubLedgerService
	StubViewService
}
