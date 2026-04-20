// Package driver defines inbound ports — interfaces that the application exposes
// to driver adapters (CLI, MCP server). These are the "primary" ports in hexagonal architecture.
package driver

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// IssueService is the inbound port for issue operations.
type IssueService interface {
	List(ctx context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error)
	Get(ctx context.Context, ref string) (*domain.Issue, error)
	Create(ctx context.Context, backend string, input domain.CreateInput) (*domain.Issue, error)
	Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error)
	Search(ctx context.Context, backend, query string, limit int) ([]domain.Issue, error)
	ListChildren(ctx context.Context, ref string) ([]domain.Issue, error)
	Backends() []string
}

// DocumentService is the inbound port for document operations.
type DocumentService interface {
	ListDocuments(ctx context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error)
	CreateDocument(ctx context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error)
}

// ProjectService is the inbound port for project operations.
type ProjectService interface {
	ListProjects(ctx context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error)
	CreateProject(ctx context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error)
	UpdateProject(ctx context.Context, backend, id string, input domain.ProjectUpdateInput) (*domain.Project, error)
}

// InitiativeService is the inbound port for initiative operations.
type InitiativeService interface {
	ListInitiatives(ctx context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error)
	CreateInitiative(ctx context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error)
}

// LabelService is the inbound port for label operations.
type LabelService interface {
	ListLabels(ctx context.Context, backend string) ([]domain.Label, error)
	CreateLabel(ctx context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error)
}

// BulkService is the inbound port for bulk issue operations.
// The service handles chunking into backend-appropriate batch sizes.
type BulkService interface {
	BulkCreateIssues(ctx context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error)
	BulkUpdateIssues(ctx context.Context, backend string, inputs []domain.BulkUpdateInput) (*domain.BulkUpdateResult, error)
}

// LaunchService is the inbound port for test launch operations (Report Portal).
type LaunchService interface {
	ListLaunches(ctx context.Context, backend string, filter domain.LaunchFilter) ([]domain.Launch, error)
	GetLaunch(ctx context.Context, backend, id string) (*domain.Launch, error)
	ListTestItems(ctx context.Context, backend, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error)
	GetTestItem(ctx context.Context, backend, id string) (*domain.TestItem, error)
	UpdateDefects(ctx context.Context, backend string, updates []domain.DefectUpdate) error
}

// BuildService is the inbound port for CI build operations (Jenkins).
type BuildService interface {
	ListJobs(ctx context.Context, backend string, filter domain.JobFilter) ([]domain.Job, error)
	GetJob(ctx context.Context, backend, name string) (*domain.Job, error)
	TriggerBuild(ctx context.Context, backend, jobName string, params map[string]string) (int64, error)
	GetBuild(ctx context.Context, backend, jobName string, number int64) (*domain.Build, error)
	GetBuildLog(ctx context.Context, backend, jobName string, number int64) (string, error)
	GetTestResults(ctx context.Context, backend, jobName string, number int64) (*domain.TestResult, error)
	GetQueue(ctx context.Context, backend string) ([]domain.QueueItem, error)
	ListBuilds(ctx context.Context, backend, jobName string, limit int) ([]domain.BuildSummary, error)
	GetLastBuild(ctx context.Context, backend, jobName string) (*domain.Build, error)
	GetLastSuccessfulBuild(ctx context.Context, backend, jobName string) (*domain.Build, error)
	GetLastFailedBuild(ctx context.Context, backend, jobName string) (*domain.Build, error)
	StopBuild(ctx context.Context, backend, jobName string, number int64) error
	GetJobParameters(ctx context.Context, backend, jobName string) ([]domain.JobParameter, error)
	ListFolderJobs(ctx context.Context, backend, folderPath string) ([]domain.Job, error)
	GetUpstreamJobs(ctx context.Context, backend, jobName string) ([]domain.Job, error)
	GetDownstreamJobs(ctx context.Context, backend, jobName string) ([]domain.Job, error)
	ListArtifacts(ctx context.Context, backend, jobName string, number int64) ([]domain.BuildArtifact, error)
	GetBuildRevision(ctx context.Context, backend, jobName string, number int64) (string, error)
	GetBuildCauses(ctx context.Context, backend, jobName string, number int64) ([]domain.BuildCause, error)
	ListNodes(ctx context.Context, backend string) ([]domain.JenkinsNode, error)
	GetNode(ctx context.Context, backend, name string) (*domain.JenkinsNode, error)
	ListViews(ctx context.Context, backend string) ([]domain.JenkinsView, error)
	GetViewJobs(ctx context.Context, backend, viewName string) ([]domain.Job, error)
}

// PipelineService is the inbound port for Jenkins pipeline operations.
type PipelineService interface {
	ListPipelineRuns(ctx context.Context, backend, jobName string) ([]domain.PipelineRun, error)
	GetPipelineRun(ctx context.Context, backend, jobName, runID string) (*domain.PipelineRun, error)
	GetPendingInputs(ctx context.Context, backend, jobName, runID string) ([]domain.PipelineInput, error)
	ApproveInput(ctx context.Context, backend, jobName, runID string) error
	AbortInput(ctx context.Context, backend, jobName, runID string) error
	GetStageLog(ctx context.Context, backend, jobName, runID, nodeID string) (string, error)
}

// FieldService is the inbound port for field metadata discovery.
type FieldService interface {
	ListFields(ctx context.Context, backend string) ([]domain.Field, error)
}

// JQLService is the inbound port for raw JQL query passthrough.
type JQLService interface {
	SearchJQL(ctx context.Context, backend, jql string, limit int) ([]domain.Issue, error)
}

// PRService is the inbound port for pull request / merge request operations.
type PRService interface {
	ListPRs(ctx context.Context, backend string, filter domain.PRFilter) ([]domain.PullRequest, error)
}

// CommentService is the inbound port for comment operations.
type CommentService interface {
	ListComments(ctx context.Context, ref string) ([]domain.Comment, error)
	AddComment(ctx context.Context, ref string, input domain.CommentCreateInput) (*domain.Comment, error)
}

// StageService is the inbound port for pre-submission staging.
type StageService interface {
	StageItem(backend string, input domain.CreateInput, reason string) string
	StageList() []domain.StagedItem
	StageGet(id string) (*domain.StagedItem, error)
	StagePatch(id string, input domain.UpdateInput) (*domain.StagedItem, error)
	StageDrop(id string) error
	StagePop(id string) (*domain.StagedItem, error)
	StagePopAll() []domain.StagedItem
}

// TriageService is the inbound port for defect lifecycle triage.
type TriageService interface {
	Triage(ctx context.Context, ref string, maxDepth int) (*domain.TriageGraph, error)
	GetTriageConfig() TriageConfig
	SetTriageConfig(cfg TriageConfig)
}

// TriageConfig holds runtime-configurable triage crawl settings.
type TriageConfig struct {
	RateLimit float64  `json:"rate_limit"` // requests per second (0 = unlimited)
	AllowList []string `json:"allow_list"` // backends to recurse into (empty = all configured)
}

// LedgerService is the inbound port for cross-backend artifact queries.
type LedgerService interface {
	LedgerGet(ctx context.Context, ref string) (*domain.ArtifactRecord, error)
	LedgerList(ctx context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error)
	LedgerSearch(ctx context.Context, query string, limit int) ([]domain.ArtifactRecord, error)
	LedgerIngest(ctx context.Context, record domain.ArtifactRecord) error
	LedgerStats(ctx context.Context) (*domain.LedgerStats, error)
}

// BackendManager is the inbound port for runtime backend management.
type BackendManager interface {
	AddBackend(repo driven.IssueRepository)
	RemoveBackend(name string) bool
	ReloadConfig(configPath string) (added, removed []string, err error)
}

// BackendHealth represents the health status of a single backend.
type BackendHealth struct {
	Name         string   `json:"name"`
	Configured   bool     `json:"configured"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// HealthStatus represents the overall health of the service.
type HealthStatus struct {
	Status   string          `json:"status"`
	Backends []BackendHealth `json:"backends"`
	Warnings []string        `json:"warnings,omitempty"`
}

// HealthService is the inbound port for health check operations.
type HealthService interface {
	Health() *HealthStatus
}
