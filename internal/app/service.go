// Package app contains the application service — the hexagon's core orchestration layer.
// It implements the driver (inbound) port and delegates to driven (outbound) adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	adapterdriven "github.com/dpopsuev/emcee/internal/adapter/driven"
	"github.com/dpopsuev/emcee/internal/config"
	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driven"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

const batchSize = 50

var (
	ErrUnknownBackend      = errors.New("unknown backend")
	ErrInvalidRef          = errors.New("invalid ref")
	ErrNotSupported        = errors.New("operation not supported by backend")
	ErrTriageNotConfigured = errors.New("triage not configured: missing graph store")
)

// Service implements all driver port interfaces by routing to the appropriate repository.
type Service struct {
	repos          map[string]driven.IssueRepository
	docRepos       map[string]driven.DocumentRepository
	projRepos      map[string]driven.ProjectRepository
	initRepos      map[string]driven.InitiativeRepository
	labelRepos     map[string]driven.LabelRepository
	bulkRepos      map[string]driven.BulkIssueRepository
	commentRepos   map[string]driven.CommentRepository
	launchRepos    map[string]driven.LaunchRepository
	fieldRepos     map[string]driven.FieldRepository
	jqlRepos       map[string]driven.JQLRepository
	prRepos        map[string]driven.PRRepository
	extLinkRepos   map[string]driven.ExternalLinkRepository
	issueLinkRepos map[string]driven.IssueLinkRepository
	extractor      driven.LinkExtractor
	graphStore     driven.GraphStore
	ledger         driven.Ledger
	crawlRateLimit float64  // requests per second (0 = unlimited)
	crawlAllowList []string // backend names to recurse into (empty = all)
	stage          *StageStore
	mu             sync.RWMutex
}

// NewService creates the application service with the given repositories.
// Repositories that implement additional interfaces (DocumentRepository, etc.)
// are automatically registered for those capabilities.
func NewService(repos ...driven.IssueRepository) *Service {
	s := &Service{
		repos:          make(map[string]driven.IssueRepository, len(repos)),
		docRepos:       make(map[string]driven.DocumentRepository),
		projRepos:      make(map[string]driven.ProjectRepository),
		initRepos:      make(map[string]driven.InitiativeRepository),
		labelRepos:     make(map[string]driven.LabelRepository),
		bulkRepos:      make(map[string]driven.BulkIssueRepository),
		commentRepos:   make(map[string]driven.CommentRepository),
		launchRepos:    make(map[string]driven.LaunchRepository),
		fieldRepos:     make(map[string]driven.FieldRepository),
		jqlRepos:       make(map[string]driven.JQLRepository),
		prRepos:        make(map[string]driven.PRRepository),
		extLinkRepos:   make(map[string]driven.ExternalLinkRepository),
		issueLinkRepos: make(map[string]driven.IssueLinkRepository),
		stage:          NewStageStore(),
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
		if elr, ok := r.(driven.ExternalLinkRepository); ok {
			s.extLinkRepos[name] = elr
		}
		if ilr, ok := r.(driven.IssueLinkRepository); ok {
			s.issueLinkRepos[name] = ilr
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
	if elr, ok := r.(driven.ExternalLinkRepository); ok {
		s.extLinkRepos[name] = elr
	}
	if ilr, ok := r.(driven.IssueLinkRepository); ok {
		s.issueLinkRepos[name] = ilr
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
	delete(s.extLinkRepos, name)
	delete(s.issueLinkRepos, name)
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
	issues, err := r.List(ctx, filter)
	if err == nil && s.ledger != nil {
		for i := range issues {
			_ = s.ledger.Put(ctx, issueToRecord(backend, &issues[i]))
		}
	}
	return issues, err
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
	if elr, ok := s.extLinkRepos[backend]; ok {
		if links, lerr := elr.ListExternalLinks(ctx, key); lerr == nil {
			issue.ExternalLinks = links
		}
	}
	if s.ledger != nil {
		_ = s.ledger.Put(ctx, issueToRecord(backend, issue))
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
	issues, err := r.Search(ctx, query, limit)
	if err == nil && s.ledger != nil {
		for i := range issues {
			_ = s.ledger.Put(ctx, issueToRecord(backend, &issues[i]))
		}
	}
	return issues, err
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
		if _, ok := s.extLinkRepos[name]; ok {
			caps = append(caps, "external_links")
		}
		if _, ok := s.issueLinkRepos[name]; ok {
			caps = append(caps, "issue_links")
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

func (s *Service) GetTestItems(ctx context.Context, backend string, ids []string) ([]domain.TestItem, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.GetTestItems(ctx, ids)
}

func (s *Service) UpdateDefects(ctx context.Context, backend string, updates []domain.DefectUpdate) error {
	r, ok := s.launchRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "launches")
	}
	return r.UpdateDefects(ctx, updates)
}

// --- Issue link operations ---

func (s *Service) LinkIssue(ctx context.Context, backend string, input domain.IssueLinkInput) error {
	r, ok := s.issueLinkRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "issue_links")
	}
	return r.CreateIssueLink(ctx, input)
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

// --- Service options ---

// ServiceOption configures optional dependencies on the Service.
type ServiceOption func(*Service)

// WithLinkExtractor injects a LinkExtractor for triage.
func WithLinkExtractor(e driven.LinkExtractor) ServiceOption {
	return func(s *Service) { s.extractor = e }
}

// WithGraphStore injects a GraphStore for triage.
func WithGraphStore(g driven.GraphStore) ServiceOption {
	return func(s *Service) { s.graphStore = g }
}

// WithLedger injects a Ledger for artifact record tracking.
func WithLedger(l driven.Ledger) ServiceOption {
	return func(s *Service) { s.ledger = l }
}

// WithCrawlRateLimit sets the max requests per second during triage crawl.
func WithCrawlRateLimit(rps float64) ServiceOption {
	return func(s *Service) { s.crawlRateLimit = rps }
}

// WithCrawlAllowList restricts triage crawl to only recurse into these backends.
func WithCrawlAllowList(backends ...string) ServiceOption {
	return func(s *Service) { s.crawlAllowList = backends }
}

// GetTriageConfig returns the current triage crawl settings.
func (s *Service) GetTriageConfig() driver.TriageConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return driver.TriageConfig{
		RateLimit: s.crawlRateLimit,
		AllowList: s.crawlAllowList,
	}
}

// SetTriageConfig updates triage crawl settings at runtime.
func (s *Service) SetTriageConfig(cfg driver.TriageConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crawlRateLimit = cfg.RateLimit
	s.crawlAllowList = cfg.AllowList
}

// Apply applies options to the service. Used to inject optional dependencies after construction.
func (s *Service) Apply(opts ...ServiceOption) {
	for _, o := range opts {
		o(s)
	}
}

// --- Ledger ---

var ErrLedgerNotConfigured = errors.New("ledger not configured")

// LedgerGet returns a single artifact record by ref.
func (s *Service) LedgerGet(ctx context.Context, ref string) (*domain.ArtifactRecord, error) {
	if s.ledger == nil {
		return nil, ErrLedgerNotConfigured
	}
	return s.ledger.Get(ctx, ref)
}

// LedgerList returns artifact records matching the filter.
func (s *Service) LedgerList(ctx context.Context, filter domain.LedgerFilter) ([]domain.ArtifactRecord, error) {
	if s.ledger == nil {
		return nil, ErrLedgerNotConfigured
	}
	return s.ledger.List(ctx, filter)
}

// LedgerSearch performs full-text search across all ledger artifacts.
func (s *Service) LedgerSearch(ctx context.Context, query string, limit int) ([]domain.ArtifactRecord, error) {
	if s.ledger == nil {
		return nil, ErrLedgerNotConfigured
	}
	return s.ledger.Search(ctx, query, limit)
}

// LedgerSimilar finds artifacts similar to the given ref.
func (s *Service) LedgerSimilar(ctx context.Context, ref string, limit int) ([]domain.ArtifactRecord, error) {
	if s.ledger == nil {
		return nil, ErrLedgerNotConfigured
	}
	return s.ledger.Similar(ctx, ref, limit)
}

// LedgerIngest actively deposits an artifact record into the ledger.
func (s *Service) LedgerIngest(ctx context.Context, record domain.ArtifactRecord) error {
	if s.ledger == nil {
		return ErrLedgerNotConfigured
	}
	return s.ledger.Put(ctx, record)
}

// LedgerStats returns aggregate ledger statistics.
func (s *Service) LedgerStats(ctx context.Context) (*domain.LedgerStats, error) {
	if s.ledger == nil {
		return nil, ErrLedgerNotConfigured
	}
	return s.ledger.Stats(ctx)
}

// issueToRecord converts an Issue to an ArtifactRecord for ledger deposit.
func issueToRecord(backend string, issue *domain.Issue) domain.ArtifactRecord {
	text := issue.Description
	for _, c := range issue.Comments {
		text += "\n" + c.Body
	}
	ref := issue.Ref
	if ref == "" {
		ref = backend + ":" + issue.Key
	}
	return domain.ArtifactRecord{
		Ref:        ref,
		Backend:    extractBackend(ref),
		Type:       "issue",
		Title:      issue.Title,
		URL:        issue.URL,
		Status:     string(issue.Status),
		Labels:     issue.Labels,
		Components: issue.Components,
		Text:       text,
		SeenAt:     time.Now(),
		UpdatedAt:  issue.UpdatedAt,
	}
}

// extractBackend returns the backend portion of a "backend:key" ref.
func extractBackend(ref string) string {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ref
}

// --- Triage ---

// Triage returns the defect lifecycle graph reachable from a seed artifact.
// It crawls recursively: fetch artifact → extract cross-refs → resolve each → repeat up to maxDepth.
// Rate-limited and filtered by allowlist.
func (s *Service) Triage(ctx context.Context, ref string, maxDepth int) (*domain.TriageGraph, error) {
	if s.graphStore == nil {
		return nil, ErrTriageNotConfigured
	}

	var limiter *rate.Limiter
	if s.crawlRateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(s.crawlRateLimit), 1)
	}

	visited := make(map[string]bool)
	s.triageCrawl(ctx, ref, 0, maxDepth, visited, limiter)

	return s.graphStore.GetGraph(ctx, ref, maxDepth)
}

// triageCrawl recursively fetches an artifact, extracts cross-refs, and recurses.
func (s *Service) triageCrawl(ctx context.Context, ref string, depth, maxDepth int, visited map[string]bool, limiter *rate.Limiter) {
	if depth > maxDepth || visited[ref] {
		return
	}
	visited[ref] = true

	// Rate limit
	if limiter != nil {
		_ = limiter.Wait(ctx)
	}

	node, text := s.triageFetchNode(ctx, ref)
	if node == nil {
		return
	}
	_ = s.graphStore.PutNode(ctx, *node)

	if s.extractor == nil || text == "" {
		return
	}

	refs, _ := s.extractor.Extract(ctx, text)
	for _, cr := range refs {
		if cr.Ref == ref {
			continue
		}
		edge := domain.TriageEdge{
			From:       ref,
			To:         cr.Ref,
			Type:       "mentions",
			Confidence: cr.Confidence,
			Source:     cr.Source,
		}
		_ = s.graphStore.PutEdge(ctx, edge)

		// Allowlist check — only recurse into allowed backends
		if !s.crawlAllowed(cr.Ref) {
			continue
		}

		s.triageCrawl(ctx, cr.Ref, depth+1, maxDepth, visited, limiter)
	}
}

// crawlAllowed checks if a ref's backend is in the allowlist.
// Empty allowlist means allow all configured backends.
func (s *Service) crawlAllowed(ref string) bool {
	backend, _, err := ParseRef(ref)
	if err != nil {
		return false
	}

	if len(s.crawlAllowList) == 0 {
		// Default: allow if backend is configured
		_, ok := s.repos[backend]
		return ok
	}

	for _, allowed := range s.crawlAllowList {
		if allowed == backend {
			return true
		}
	}
	return false
}

// triageFetchNode fetches an artifact by ref and returns its node + text content for extraction.
// Dispatches to the correct port based on ref format.
func (s *Service) triageFetchNode(ctx context.Context, ref string) (node *domain.TriageNode, text string) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, ""
	}

	// Try issue first (most common)
	if r, ok := s.repos[backend]; ok {
		issue, issueErr := r.Get(ctx, key)
		if issueErr == nil {
			var b strings.Builder
			b.WriteString(issue.Description)
			for _, c := range issue.Comments {
				b.WriteString("\n")
				b.WriteString(c.Body)
			}
			return &domain.TriageNode{
				Ref:       ref,
				Type:      "issue",
				Phase:     "stored",
				Title:     issue.Title,
				URL:       issue.URL,
				Status:    string(issue.Status),
				Timestamp: issue.CreatedAt,
			}, b.String()
		}
	}

	// Try launch (reportportal:launch/N)
	if lr, ok := s.launchRepos[backend]; ok && strings.HasPrefix(key, "launch/") {
		launchID := strings.TrimPrefix(key, "launch/")
		launch, launchErr := lr.GetLaunch(ctx, launchID)
		if launchErr == nil {
			return &domain.TriageNode{
				Ref:       ref,
				Type:      "launch",
				Phase:     "detected",
				Title:     launch.Name,
				URL:       launch.URL,
				Status:    launch.Status,
				Timestamp: launch.StartTime,
			}, launch.Description
		}
	}

	// Try PR (github:org/repo#N or gitlab:path!N)
	if pr, ok := s.prRepos[backend]; ok {
		prs, prErr := pr.ListPRs(ctx, domain.PRFilter{Limit: 1})
		if prErr == nil && len(prs) > 0 {
			return &domain.TriageNode{
				Ref:   ref,
				Type:  "pr",
				Phase: "fixed",
			}, ""
		}
	}

	// Unknown — store as placeholder leaf node
	return &domain.TriageNode{
		Ref:  ref,
		Type: "unknown",
	}, ""
}
