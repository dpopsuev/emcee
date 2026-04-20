// Package app contains the application service — the hexagon's core orchestration layer.
// It implements the driver (inbound) port and delegates to driven (outbound) adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driver"
)

const batchSize = 50

var (
	ErrUnknownBackend = errors.New("unknown backend")
	ErrInvalidRef     = errors.New("invalid ref")
	ErrNotSupported   = errors.New("operation not supported by backend")
)

// Service implements all driver port interfaces by routing to the appropriate repository.
type Service struct {
	repos        map[string]driven.IssueRepository
	docRepos     map[string]driven.DocumentRepository
	projRepos    map[string]driven.ProjectRepository
	initRepos    map[string]driven.InitiativeRepository
	labelRepos   map[string]driven.LabelRepository
	bulkRepos    map[string]driven.BulkIssueRepository
	commentRepos map[string]driven.CommentRepository
	launchRepos  map[string]driven.LaunchRepository
	fieldRepos   map[string]driven.FieldRepository
	jqlRepos     map[string]driven.JQLRepository
	prRepos      map[string]driven.PRRepository
	buildRepos   map[string]driven.BuildRepository
	stage        *StageStore
	mu           sync.RWMutex
}

// NewService creates the application service with the given repositories.
// Repositories that implement additional interfaces (DocumentRepository, etc.)
// are automatically registered for those capabilities.
func NewService(repos ...driven.IssueRepository) *Service {
	s := &Service{
		repos:        make(map[string]driven.IssueRepository, len(repos)),
		docRepos:     make(map[string]driven.DocumentRepository),
		projRepos:    make(map[string]driven.ProjectRepository),
		initRepos:    make(map[string]driven.InitiativeRepository),
		labelRepos:   make(map[string]driven.LabelRepository),
		bulkRepos:    make(map[string]driven.BulkIssueRepository),
		commentRepos: make(map[string]driven.CommentRepository),
		launchRepos:  make(map[string]driven.LaunchRepository),
		fieldRepos:   make(map[string]driven.FieldRepository),
		jqlRepos:     make(map[string]driven.JQLRepository),
		prRepos:      make(map[string]driven.PRRepository),
		buildRepos:   make(map[string]driven.BuildRepository),
		stage:        NewStageStore(),
	}
	for _, r := range repos {
		name := r.Name()
		s.repos[name] = r
		if dr, ok := r.(driven.DocumentRepository); ok {
			s.docRepos[name] = dr
		}
		if pr, ok := r.(driven.ProjectRepository); ok {
			s.projRepos[name] = pr
		}
		if ir, ok := r.(driven.InitiativeRepository); ok {
			s.initRepos[name] = ir
		}
		if lr, ok := r.(driven.LabelRepository); ok {
			s.labelRepos[name] = lr
		}
		if br, ok := r.(driven.BulkIssueRepository); ok {
			s.bulkRepos[name] = br
		}
		if cr, ok := r.(driven.CommentRepository); ok {
			s.commentRepos[name] = cr
		}
		if lr, ok := r.(driven.LaunchRepository); ok {
			s.launchRepos[name] = lr
		}
		if fr, ok := r.(driven.FieldRepository); ok {
			s.fieldRepos[name] = fr
		}
		if jr, ok := r.(driven.JQLRepository); ok {
			s.jqlRepos[name] = jr
		}
		if pr, ok := r.(driven.PRRepository); ok {
			s.prRepos[name] = pr
		}
		if br, ok := r.(driven.BuildRepository); ok {
			s.buildRepos[name] = br
		}
	}
	return s
}

// AddBackend registers a new backend at runtime. Thread-safe.
func (s *Service) AddBackend(r driven.IssueRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := r.Name()
	s.repos[name] = r
	if dr, ok := r.(driven.DocumentRepository); ok {
		s.docRepos[name] = dr
	}
	if pr, ok := r.(driven.ProjectRepository); ok {
		s.projRepos[name] = pr
	}
	if ir, ok := r.(driven.InitiativeRepository); ok {
		s.initRepos[name] = ir
	}
	if lr, ok := r.(driven.LabelRepository); ok {
		s.labelRepos[name] = lr
	}
	if br, ok := r.(driven.BulkIssueRepository); ok {
		s.bulkRepos[name] = br
	}
	if cr, ok := r.(driven.CommentRepository); ok {
		s.commentRepos[name] = cr
	}
	if lr, ok := r.(driven.LaunchRepository); ok {
		s.launchRepos[name] = lr
	}
	if fr, ok := r.(driven.FieldRepository); ok {
		s.fieldRepos[name] = fr
	}
	if jr, ok := r.(driven.JQLRepository); ok {
		s.jqlRepos[name] = jr
	}
	if pr, ok := r.(driven.PRRepository); ok {
		s.prRepos[name] = pr
	}
	if br, ok := r.(driven.BuildRepository); ok {
		s.buildRepos[name] = br
	}
}

// RemoveBackend removes a backend by name at runtime. Thread-safe.
func (s *Service) RemoveBackend(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.repos[name]; !ok {
		return false
	}
	delete(s.repos, name)
	delete(s.docRepos, name)
	delete(s.projRepos, name)
	delete(s.initRepos, name)
	delete(s.labelRepos, name)
	delete(s.bulkRepos, name)
	delete(s.commentRepos, name)
	delete(s.launchRepos, name)
	delete(s.fieldRepos, name)
	delete(s.jqlRepos, name)
	delete(s.prRepos, name)
	delete(s.buildRepos, name)
	return true
}

// ReloadConfig re-reads the config file, diffs against current backends,
// adds new ones and removes stale ones. Returns names of added/removed backends.
func (s *Service) ReloadConfig(configPath string) (added, removed []string, err error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reload config: %w", err)
	}

	newRepos, warnings := adapterdriven.CreateFromConfig(cfg)
	for _, w := range warnings {
		removed = append(removed, "warning: "+w)
	}

	// Build set of new backend names
	newNames := make(map[string]bool, len(newRepos))
	for _, r := range newRepos {
		newNames[r.Name()] = true
	}

	// Remove backends no longer in config
	current := s.Backends()
	for _, name := range current {
		if !newNames[name] {
			s.RemoveBackend(name)
			removed = append(removed, name)
		}
	}

	// Add new/updated backends
	currentSet := make(map[string]bool, len(current))
	for _, name := range current {
		currentSet[name] = true
	}
	for _, r := range newRepos {
		if !currentSet[r.Name()] {
			s.AddBackend(r)
			added = append(added, r.Name())
		}
	}

	return added, removed, nil
}

// ParseRef splits "linear:HEG-17" into backend and key.
func ParseRef(ref string) (backend, key string, err error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %q (expected backend:key, e.g. linear:HEG-17)", ErrInvalidRef, ref)
	}
	return parts[0], parts[1], nil
}

func (s *Service) repo(name string) (driven.IssueRepository, error) {
	s.mu.RLock()
	r, ok := s.repos[name]
	s.mu.RUnlock()
	if !ok {
		return nil, s.unknownBackendErr(name)
	}
	return r, nil
}

func (s *Service) unknownBackendErr(name string) error {
	available := make([]string, 0, len(s.repos))
	for k := range s.repos {
		available = append(available, k)
	}

	if len(available) == 0 {
		return fmt.Errorf("%w: %q - no backends configured\n\nTo fix:\n  1. Set environment variable for %s:\n     - LINEAR_API_KEY for Linear\n     - JIRA_API_TOKEN, JIRA_URL, JIRA_EMAIL for Jira\n     - GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO for GitHub\n     - GITLAB_TOKEN, GITLAB_PROJECT for GitLab\n  2. Or create ~/.config/emcee/config.yaml with backend configuration\n\nGet API keys at:\n  - Linear: https://linear.app/settings/api\n  - GitHub: https://github.com/settings/tokens\n  - Jira: https://id.atlassian.com/manage-profile/security/api-tokens\n  - GitLab: https://gitlab.com/-/user_settings/personal_access_tokens",
			ErrUnknownBackend, name, name)
	}

	return fmt.Errorf("%w: %q (available: %s)", ErrUnknownBackend, name, strings.Join(available, ", "))
}

func (s *Service) notSupportedErr(backend, op string) error {
	return fmt.Errorf("%w: %q does not support %s", ErrNotSupported, backend, op)
}

// --- Issue operations ---

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
	issue, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cr, ok := s.commentRepos[backend]; ok {
		if comments, cerr := cr.ListComments(ctx, key); cerr == nil {
			issue.Comments = comments
		}
	}
	return issue, nil
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

func (s *Service) Search(ctx context.Context, backend, query string, limit int) ([]domain.Issue, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Search(ctx, query, limit)
}

func (s *Service) ListChildren(ctx context.Context, ref string) ([]domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.ListChildren(ctx, key)
}

func (s *Service) Backends() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.repos))
	for k := range s.repos {
		names = append(names, k)
	}
	return names
}

// Health returns the current health status of all backends.
func (s *Service) Health() *driver.HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := &driver.HealthStatus{
		Status:   "healthy",
		Backends: make([]driver.BackendHealth, 0),
	}

	// Check configured backends and detect capabilities
	for name := range s.repos {
		caps := []string{"issues"}
		if _, ok := s.docRepos[name]; ok {
			caps = append(caps, "documents")
		}
		if _, ok := s.projRepos[name]; ok {
			caps = append(caps, "projects")
		}
		if _, ok := s.initRepos[name]; ok {
			caps = append(caps, "initiatives")
		}
		if _, ok := s.labelRepos[name]; ok {
			caps = append(caps, "labels")
		}
		if _, ok := s.bulkRepos[name]; ok {
			caps = append(caps, "bulk")
		}
		if _, ok := s.commentRepos[name]; ok {
			caps = append(caps, "comments")
		}
		if _, ok := s.launchRepos[name]; ok {
			caps = append(caps, "launches")
		}
		if _, ok := s.fieldRepos[name]; ok {
			caps = append(caps, "fields")
		}
		if _, ok := s.jqlRepos[name]; ok {
			caps = append(caps, "jql")
		}
		if _, ok := s.prRepos[name]; ok {
			caps = append(caps, "prs")
		}
		if _, ok := s.buildRepos[name]; ok {
			caps = append(caps, "builds")
		}
		status.Backends = append(status.Backends, driver.BackendHealth{
			Name:         name,
			Configured:   true,
			Status:       "healthy",
			Capabilities: caps,
		})
	}

	// If no backends configured, set to degraded
	if len(s.repos) == 0 {
		status.Status = "degraded"
		status.Warnings = append(status.Warnings, "No backends configured. Set LINEAR_API_KEY, JIRA_API_TOKEN, GITHUB_TOKEN, or GITLAB_TOKEN environment variables, or create ~/.config/emcee/config.yaml")
	}

	return status
}

// --- Document operations ---

func (s *Service) ListDocuments(ctx context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error) {
	r, ok := s.docRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "documents")
	}
	return r.ListDocuments(ctx, filter)
}

func (s *Service) CreateDocument(ctx context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error) {
	r, ok := s.docRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "documents")
	}
	return r.CreateDocument(ctx, input)
}

// --- Project operations ---

func (s *Service) ListProjects(ctx context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error) {
	r, ok := s.projRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "projects")
	}
	return r.ListProjects(ctx, filter)
}

func (s *Service) CreateProject(ctx context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error) {
	r, ok := s.projRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "projects")
	}
	return r.CreateProject(ctx, input)
}

func (s *Service) UpdateProject(ctx context.Context, backend, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	r, ok := s.projRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "projects")
	}
	return r.UpdateProject(ctx, id, input)
}

// --- Initiative operations ---

func (s *Service) ListInitiatives(ctx context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	r, ok := s.initRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "initiatives")
	}
	return r.ListInitiatives(ctx, filter)
}

func (s *Service) CreateInitiative(ctx context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	r, ok := s.initRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "initiatives")
	}
	return r.CreateInitiative(ctx, input)
}

// --- Label operations ---

func (s *Service) ListLabels(ctx context.Context, backend string) ([]domain.Label, error) {
	r, ok := s.labelRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "labels")
	}
	return r.ListLabels(ctx)
}

func (s *Service) CreateLabel(ctx context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error) {
	r, ok := s.labelRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "labels")
	}
	return r.CreateLabel(ctx, input)
}

// --- Bulk operations ---

func (s *Service) BulkCreateIssues(ctx context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error) {
	r, ok := s.bulkRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "bulk create")
	}

	result := &domain.BulkCreateResult{Total: len(inputs)}

	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		result.Batches++

		created, err := r.BulkCreateIssues(ctx, inputs[i:end])
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("batch %d: %v", result.Batches, err))
			continue
		}
		result.Created = append(result.Created, created...)
	}
	return result, nil
}

// --- Comment operations ---

func (s *Service) ListComments(ctx context.Context, ref string) ([]domain.Comment, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, ok := s.commentRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "comments")
	}
	return r.ListComments(ctx, key)
}

func (s *Service) AddComment(ctx context.Context, ref string, input domain.CommentCreateInput) (*domain.Comment, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, ok := s.commentRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "comments")
	}
	return r.AddComment(ctx, key, input)
}

// --- Field discovery ---

func (s *Service) ListFields(ctx context.Context, backend string) ([]domain.Field, error) {
	r, ok := s.fieldRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "fields")
	}
	return r.ListFields(ctx)
}

// --- JQL passthrough ---

func (s *Service) SearchJQL(ctx context.Context, backend, jql string, limit int) ([]domain.Issue, error) {
	r, ok := s.jqlRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "jql")
	}
	return r.SearchJQL(ctx, jql, limit)
}

// --- PR/MR operations ---

func (s *Service) ListPRs(ctx context.Context, backend string, filter domain.PRFilter) ([]domain.PullRequest, error) {
	r, ok := s.prRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "pull requests")
	}
	return r.ListPRs(ctx, filter)
}

// --- Launch operations (Report Portal) ---

func (s *Service) ListLaunches(ctx context.Context, backend string, filter domain.LaunchFilter) ([]domain.Launch, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.ListLaunches(ctx, filter)
}

func (s *Service) GetLaunch(ctx context.Context, backend, id string) (*domain.Launch, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.GetLaunch(ctx, id)
}

func (s *Service) ListTestItems(ctx context.Context, backend, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.ListTestItems(ctx, launchID, filter)
}

func (s *Service) GetTestItem(ctx context.Context, backend, id string) (*domain.TestItem, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.GetTestItem(ctx, id)
}

func (s *Service) UpdateDefects(ctx context.Context, backend string, updates []domain.DefectUpdate) error {
	r, ok := s.launchRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "launches")
	}
	return r.UpdateDefects(ctx, updates)
}

// --- Build operations (Jenkins) ---

func (s *Service) ListJobs(ctx context.Context, backend string, filter domain.JobFilter) ([]domain.Job, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.ListJobs(ctx, filter)
}

func (s *Service) GetJob(ctx context.Context, backend, name string) (*domain.Job, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetJob(ctx, name)
}

func (s *Service) TriggerBuild(ctx context.Context, backend, jobName string, params map[string]string) (int64, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return 0, s.notSupportedErr(backend, "builds")
	}
	return r.TriggerBuild(ctx, jobName, params)
}

func (s *Service) GetBuild(ctx context.Context, backend, jobName string, number int64) (*domain.Build, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetBuild(ctx, jobName, number)
}

func (s *Service) GetBuildLog(ctx context.Context, backend, jobName string, number int64) (string, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return "", s.notSupportedErr(backend, "builds")
	}
	return r.GetBuildLog(ctx, jobName, number)
}

func (s *Service) GetTestResults(ctx context.Context, backend, jobName string, number int64) (*domain.TestResult, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetTestResults(ctx, jobName, number)
}

func (s *Service) GetQueue(ctx context.Context, backend string) ([]domain.QueueItem, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetQueue(ctx)
}

func (s *Service) ListBuilds(ctx context.Context, backend, jobName string, limit int) ([]domain.BuildSummary, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.ListBuilds(ctx, jobName, limit)
}

func (s *Service) GetLastBuild(ctx context.Context, backend, jobName string) (*domain.Build, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetLastBuild(ctx, jobName)
}

func (s *Service) GetLastSuccessfulBuild(ctx context.Context, backend, jobName string) (*domain.Build, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetLastSuccessfulBuild(ctx, jobName)
}

func (s *Service) GetLastFailedBuild(ctx context.Context, backend, jobName string) (*domain.Build, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetLastFailedBuild(ctx, jobName)
}

func (s *Service) StopBuild(ctx context.Context, backend, jobName string, number int64) error {
	r, ok := s.buildRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "builds")
	}
	return r.StopBuild(ctx, jobName, number)
}

func (s *Service) GetJobParameters(ctx context.Context, backend, jobName string) ([]domain.JobParameter, error) {
	r, ok := s.buildRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "builds")
	}
	return r.GetJobParameters(ctx, jobName)
}

// --- Stage operations ---

func (s *Service) StageItem(backend string, input domain.CreateInput, reason string) string {
	return s.stage.StageItem(backend, input, reason)
}

func (s *Service) StageList() []domain.StagedItem {
	return s.stage.StageList()
}

func (s *Service) StageGet(id string) (*domain.StagedItem, error) {
	return s.stage.StageGet(id)
}

func (s *Service) StagePatch(id string, input domain.UpdateInput) (*domain.StagedItem, error) {
	return s.stage.StagePatch(id, input)
}

func (s *Service) StageDrop(id string) error {
	return s.stage.StageDrop(id)
}

func (s *Service) StagePop(id string) (*domain.StagedItem, error) {
	return s.stage.StagePop(id)
}

func (s *Service) StagePopAll() []domain.StagedItem {
	return s.stage.StagePopAll()
}

func (s *Service) BulkUpdateIssues(ctx context.Context, backend string, inputs []domain.BulkUpdateInput) (*domain.BulkUpdateResult, error) {
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}

	result := &domain.BulkUpdateResult{Total: len(inputs)}
	for _, input := range inputs {
		_, key, refErr := ParseRef(input.Ref)
		if refErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", input.Ref, refErr))
			continue
		}
		updateInput := domain.UpdateInput{
			Title:       input.Title,
			Description: input.Description,
			Status:      input.Status,
			Priority:    input.Priority,
		}
		issue, updateErr := r.Update(ctx, key, updateInput)
		if updateErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", input.Ref, updateErr))
			continue
		}
		result.Updated = append(result.Updated, *issue)
	}
	return result, nil
}
