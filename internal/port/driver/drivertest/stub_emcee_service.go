package drivertest

import (
	mcpdriver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
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
	StubLaunchService
	StubFieldService
	StubJQLService
	StubPRService
	StubBuildService
	StubPipelineService
	StubBackendManager
	StubTriageService
	StubLedgerService
}
