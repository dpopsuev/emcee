// Package app contains the application service — the hexagon's core orchestration layer.
// It implements the driver (inbound) port and delegates to driven (outbound) adapters.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/dpopsuev/emcee/internal/config"
	"github.com/dpopsuev/emcee/internal/domain"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/manifest"
	"github.com/dpopsuev/emcee/internal/poller"
	"github.com/dpopsuev/emcee/internal/repository"
	"github.com/dpopsuev/emcee/internal/service"
)

const batchSize = 50

var (
	ErrUnknownBackend      = errors.New("unknown backend")
	ErrInvalidRef          = errors.New("invalid ref")
	ErrNotSupported        = errors.New("operation not supported by backend")
	ErrBackendRequired     = errors.New("backend not specified")
	ErrTriageNotConfigured = errors.New("triage not configured: missing graph store")
	errJQLRequired         = errors.New("JQL search not supported (needed for template discovery)")
)

// Service implements all driver port interfaces by routing to the appropriate repository.
type Service struct {
	repos          map[string]repository.IssueRepository
	docRepos       map[string]repository.DocumentRepository
	projRepos      map[string]repository.ProjectRepository
	initRepos      map[string]repository.InitiativeRepository
	labelRepos     map[string]repository.LabelRepository
	bulkRepos      map[string]repository.BulkIssueRepository
	commentRepos   map[string]repository.CommentRepository
	launchRepos    map[string]repository.LaunchRepository
	fieldRepos     map[string]repository.FieldRepository
	statusRepos    map[string]repository.StatusRepository
	jqlRepos       map[string]repository.JQLRepository
	prRepos        map[string]repository.PRRepository
	extLinkRepos   map[string]repository.ExternalLinkRepository
	issueLinkRepos map[string]repository.IssueLinkRepository
	gistRepos      map[string]repository.GistRepository
	prReviewRepos  map[string]repository.PRReviewRepository
	changelogRepos map[string]repository.ChangelogRepository
	extractor      repository.LinkExtractor
	graphStore     repository.GraphStore
	deltaRepos     map[string]repository.DeltaSyncer
	watchScopes    map[string]domain.WatchScope
	ledger         repository.Ledger
	cursor         poller.Cursor
	crawlRateLimit float64  // requests per second (0 = unlimited)
	crawlAllowList []string // backend names to recurse into (empty = all)
	stage          *StageStore
	view           *ViewStore
	launchView     *LaunchViewStore
	mu             sync.RWMutex
}

// NewService creates the application service with the given repositories.
// Repositories that implement additional interfaces (DocumentRepository, etc.)
// are automatically registered for those capabilities.
//
//nolint:funlen
func NewService(repos ...repository.IssueRepository) *Service {
	s := &Service{
		repos:          make(map[string]repository.IssueRepository, len(repos)),
		docRepos:       make(map[string]repository.DocumentRepository),
		projRepos:      make(map[string]repository.ProjectRepository),
		initRepos:      make(map[string]repository.InitiativeRepository),
		labelRepos:     make(map[string]repository.LabelRepository),
		bulkRepos:      make(map[string]repository.BulkIssueRepository),
		commentRepos:   make(map[string]repository.CommentRepository),
		launchRepos:    make(map[string]repository.LaunchRepository),
		fieldRepos:     make(map[string]repository.FieldRepository),
		statusRepos:    make(map[string]repository.StatusRepository),
		jqlRepos:       make(map[string]repository.JQLRepository),
		deltaRepos:     make(map[string]repository.DeltaSyncer),
		watchScopes:    make(map[string]domain.WatchScope),
		prRepos:        make(map[string]repository.PRRepository),
		extLinkRepos:   make(map[string]repository.ExternalLinkRepository),
		issueLinkRepos: make(map[string]repository.IssueLinkRepository),
		gistRepos:      make(map[string]repository.GistRepository),
		prReviewRepos:  make(map[string]repository.PRReviewRepository),
		changelogRepos: make(map[string]repository.ChangelogRepository),
		stage:          NewStageStore(),
		view:           NewViewStore(),
		launchView:     newLaunchViewStore(),
	}
	for _, r := range repos {
		name := r.Name()
		s.repos[name] = r
		if dr, ok := r.(repository.DocumentRepository); ok {
			s.docRepos[name] = dr
		}
		if pr, ok := r.(repository.ProjectRepository); ok {
			s.projRepos[name] = pr
		}
		if ir, ok := r.(repository.InitiativeRepository); ok {
			s.initRepos[name] = ir
		}
		if lr, ok := r.(repository.LabelRepository); ok {
			s.labelRepos[name] = lr
		}
		if br, ok := r.(repository.BulkIssueRepository); ok {
			s.bulkRepos[name] = br
		}
		if cr, ok := r.(repository.CommentRepository); ok {
			s.commentRepos[name] = cr
		}
		if lr, ok := r.(repository.LaunchRepository); ok {
			s.launchRepos[name] = lr
		}
		if fr, ok := r.(repository.FieldRepository); ok {
			s.fieldRepos[name] = fr
		}
		if sr, ok := r.(repository.StatusRepository); ok {
			s.statusRepos[name] = sr
		}
		if jr, ok := r.(repository.JQLRepository); ok {
			s.jqlRepos[name] = jr
		}
		if ds, ok := r.(repository.DeltaSyncer); ok {
			s.deltaRepos[name] = ds
		}
		if pr, ok := r.(repository.PRRepository); ok {
			s.prRepos[name] = pr
		}
		if elr, ok := r.(repository.ExternalLinkRepository); ok {
			s.extLinkRepos[name] = elr
		}
		if ilr, ok := r.(repository.IssueLinkRepository); ok {
			s.issueLinkRepos[name] = ilr
		}
		if gr, ok := r.(repository.GistRepository); ok {
			s.gistRepos[name] = gr
		}
		if prr, ok := r.(repository.PRReviewRepository); ok {
			s.prReviewRepos[name] = prr
		}
		if clr, ok := r.(repository.ChangelogRepository); ok {
			s.changelogRepos[name] = clr
		}
	}
	return s
}

// AddBackend registers a new backend at runtime. Thread-safe.
func (s *Service) AddBackend(r repository.IssueRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := r.Name()
	s.repos[name] = r
	if dr, ok := r.(repository.DocumentRepository); ok {
		s.docRepos[name] = dr
	}
	if pr, ok := r.(repository.ProjectRepository); ok {
		s.projRepos[name] = pr
	}
	if ir, ok := r.(repository.InitiativeRepository); ok {
		s.initRepos[name] = ir
	}
	if lr, ok := r.(repository.LabelRepository); ok {
		s.labelRepos[name] = lr
	}
	if br, ok := r.(repository.BulkIssueRepository); ok {
		s.bulkRepos[name] = br
	}
	if cr, ok := r.(repository.CommentRepository); ok {
		s.commentRepos[name] = cr
	}
	if lr, ok := r.(repository.LaunchRepository); ok {
		s.launchRepos[name] = lr
	}
	if fr, ok := r.(repository.FieldRepository); ok {
		s.fieldRepos[name] = fr
	}
	if sr, ok := r.(repository.StatusRepository); ok {
		s.statusRepos[name] = sr
	}
	if jr, ok := r.(repository.JQLRepository); ok {
		s.jqlRepos[name] = jr
	}
	if ds, ok := r.(repository.DeltaSyncer); ok {
		s.deltaRepos[name] = ds
	}
	if pr, ok := r.(repository.PRRepository); ok {
		s.prRepos[name] = pr
	}
	if elr, ok := r.(repository.ExternalLinkRepository); ok {
		s.extLinkRepos[name] = elr
	}
	if ilr, ok := r.(repository.IssueLinkRepository); ok {
		s.issueLinkRepos[name] = ilr
	}
	if gr, ok := r.(repository.GistRepository); ok {
		s.gistRepos[name] = gr
	}
	if prr, ok := r.(repository.PRReviewRepository); ok {
		s.prReviewRepos[name] = prr
	}
	if clr, ok := r.(repository.ChangelogRepository); ok {
		s.changelogRepos[name] = clr
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
	delete(s.statusRepos, name)
	delete(s.jqlRepos, name)
	delete(s.deltaRepos, name)
	delete(s.watchScopes, name)
	delete(s.prRepos, name)
	delete(s.extLinkRepos, name)
	delete(s.issueLinkRepos, name)
	delete(s.gistRepos, name)
	delete(s.prReviewRepos, name)
	return true
}

// ReloadConfig re-reads the config file, diffs against current backends,
// adds new ones and removes stale ones. Returns names of added/removed backends.
func (s *Service) ReloadConfig(configPath string) (added, removed []string, err error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reload config: %w", err)
	}

	newRepos, warnings := infra.CreateFromConfig(cfg)
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

// ProjectKeyFromRef extracts the Jira-style project key from a ref like
// "jira:PROJ-42" → "PROJ". Returns "" if the ref is not in backend:KEY-N format.
func ProjectKeyFromRef(ref string) string {
	_, key, err := ParseRef(ref)
	if err != nil {
		return ""
	}
	idx := strings.LastIndex(key, "-")
	if idx <= 0 {
		return ""
	}
	suffix := key[idx+1:]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return key[:idx]
}

func (s *Service) repo(name string) (repository.IssueRepository, error) {
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

func (s *Service) DefaultProject(backend string) string {
	r, err := s.repo(backend)
	if err != nil {
		return ""
	}
	if ps, ok := r.(repository.ProjectScoper); ok {
		return ps.DefaultProject()
	}
	return ""
}

func (s *Service) SetDefaultProject(backend, project string) error {
	r, err := s.repo(backend)
	if err != nil {
		return err
	}
	ps, ok := r.(repository.ProjectScoper)
	if !ok {
		return s.notSupportedErr(backend, "set_default_project")
	}
	ps.SetDefaultProject(project)
	return nil
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
	if input.ParentID != "" {
		input.ParentID = s.normalizeParentID(input.ParentID, &input.ProjectID)
	}
	r, err := s.repo(backend)
	if err != nil {
		return nil, err
	}
	return r.Create(ctx, input)
}

// normalizeParentID strips a ref prefix (jira:PROJ-42 → PROJ-42) and infers
// the project key into projectID when it is empty.
func (s *Service) normalizeParentID(parentID string, projectID *string) string {
	_, key, err := ParseRef(parentID)
	if err != nil {
		return parentID
	}
	if *projectID == "" {
		if pk := ProjectKeyFromRef(parentID); pk != "" {
			*projectID = pk
		}
	}
	return key
}

func (s *Service) Update(ctx context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil {
		if _, pkey, perr := ParseRef(*input.ParentID); perr == nil {
			input.ParentID = &pkey
		}
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

// DiscoverTemplate samples recent issues from a project+issueType and
// extracts the common description template by finding section headers
// that appear in all sampled descriptions.
func (s *Service) DiscoverTemplate(ctx context.Context, backend, project, issueType string, sampleSize int) (*domain.Template, error) {
	jr, ok := s.jqlRepos[backend]
	if !ok {
		return nil, fmt.Errorf("backend %q: %w", backend, errJQLRequired)
	}
	if sampleSize <= 0 {
		sampleSize = 5
	}
	jql := fmt.Sprintf("project = %s AND issuetype = %s ORDER BY created DESC", project, issueType)
	issues, err := jr.SearchJQL(ctx, jql, sampleSize)
	if err != nil {
		return nil, err
	}
	var descs []string
	for _, issue := range issues {
		if issue.Description != "" {
			descs = append(descs, issue.Description)
		}
	}
	sections := domain.ExtractTemplateSections(descs)
	if len(sections) == 0 {
		return nil, nil
	}
	return &domain.Template{
		Project:   project,
		IssueType: issueType,
		Sections:  sections,
		Body:      domain.BuildTemplateBody(sections),
	}, nil
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
func (s *Service) Health() *service.HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := &service.HealthStatus{
		Status:   "healthy",
		Backends: make([]service.BackendHealth, 0),
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
		status.Backends = append(status.Backends, service.BackendHealth{
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

func (s *Service) ListChangelog(ctx context.Context, ref string, limit int) ([]domain.ChangelogEntry, error) {
	backend, key, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	r, ok := s.changelogRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "changelog")
	}
	return r.ListChangelog(ctx, key, limit)
}

func (s *Service) DiscoverFields(ctx context.Context, backend, configDir string) (map[string]string, error) {
	r, ok := s.fieldRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "fields")
	}
	domainFields, err := r.ListFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("list fields for discovery: %w", err)
	}
	mappings := make(map[string]string, len(domainFields))
	for _, f := range domainFields {
		if f.Custom {
			mappings[f.Name] = f.ID
		}
	}
	m := manifest.Discover(backend, mappings)
	if err := manifest.Save(manifest.DefaultKind, backend, configDir, m); err != nil {
		return nil, fmt.Errorf("save field manifest: %w", err)
	}
	return m.Mappings, nil
}

func (s *Service) DiscoverStatuses(ctx context.Context, backend, configDir string) (map[string]string, error) {
	r, ok := s.statusRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "statuses")
	}
	entries, err := r.ListStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list statuses for discovery: %w", err)
	}
	mappings := make(map[string]string, len(entries))
	for _, e := range entries {
		mappings[e.Name] = mapCategoryToStatus(e.CategoryKey)
	}
	m := manifest.Discover(backend, mappings)
	if err := manifest.Save("statuses", backend, configDir, m); err != nil {
		return nil, fmt.Errorf("save status manifest: %w", err)
	}
	return m.Mappings, nil
}

func mapCategoryToStatus(categoryKey string) string {
	switch categoryKey {
	case "new":
		return string(domain.StatusTodo)
	case "indeterminate":
		return string(domain.StatusInProgress)
	case "done":
		return string(domain.StatusDone)
	default:
		return string(domain.StatusBacklog)
	}
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
	if backend == "" {
		backend = s.inferPRBackend()
		if backend == "" {
			return nil, s.prBackendRequiredErr()
		}
	}
	r, ok := s.prRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "pull requests")
	}
	return r.ListPRs(ctx, filter)
}

// inferPRBackend returns the sole PR-capable backend name when exactly one is
// configured, enabling backend-free calls like prs(repo="owner/repo").
func (s *Service) inferPRBackend() string {
	if len(s.prRepos) == 1 {
		for name := range s.prRepos {
			return name
		}
	}
	return ""
}

// prBackendRequiredErr builds an actionable error that lists the available
// PR-capable backends (or advises the user to configure one).
func (s *Service) prBackendRequiredErr() error {
	if len(s.prRepos) == 0 {
		return fmt.Errorf("%w: no PR-capable backends are configured — add a GitHub or GitLab backend first", ErrBackendRequired)
	}
	names := make([]string, 0, len(s.prRepos))
	for name := range s.prRepos {
		names = append(names, "backend="+name)
	}
	return fmt.Errorf("%w: use %s to query pull requests", ErrBackendRequired, strings.Join(names, " or "))
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
	ref := backend + ":" + launchID
	if items, ok := s.launchView.GetItems(ref, filter.Status); ok {
		if filter.Limit > 0 && len(items) > filter.Limit {
			items = items[:filter.Limit]
		}
		return items, nil
	}
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

func (s *Service) SearchTestItems(ctx context.Context, backend string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}

	// Resolve launch IDs from name / date-range when not given explicitly.
	if len(filter.LaunchIDs) == 0 {
		launches, err := r.ListLaunches(ctx, domain.LaunchFilter{
			Name:        filter.LaunchName,
			StartAfter:  filter.Since,
			StartBefore: filter.Before,
			Attributes:  filter.LaunchAttributes,
			Limit:       200,
		})
		if err != nil {
			return nil, fmt.Errorf("resolving launches for search_items: %w", err)
		}
		if len(launches) == 0 {
			return nil, nil
		}
		for _, l := range launches {
			filter.LaunchIDs = append(filter.LaunchIDs, l.ID)
		}
	}

	// RP does not support cross-launch item queries without a saved filter.
	// Fan out: query each launch individually and aggregate results.
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	perLaunch := domain.TestItemFilter{
		Name:        filter.Name,
		Status:      filter.Status,
		IssueType:   filter.IssueType,
		Limit:       limit,
		IncludeLogs: filter.IncludeLogs,
	}
	var all []domain.TestItem
	for _, launchID := range filter.LaunchIDs {
		items, err := r.ListTestItems(ctx, launchID, perLaunch)
		if err != nil {
			return nil, fmt.Errorf("items for launch %s: %w", launchID, err)
		}
		all = append(all, items...)
		if len(all) >= limit {
			break
		}
	}
	return all, nil
}

func (s *Service) UpdateDefects(ctx context.Context, backend string, updates []domain.DefectUpdate) error {
	r, ok := s.launchRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "launches")
	}
	return r.UpdateDefects(ctx, updates)
}

// --- PR review operations ---

func (s *Service) ListPRReviews(ctx context.Context, backend string, prNumber int) ([]domain.PRReview, error) {
	if backend == "" {
		backend = s.inferPRReviewBackend()
		if backend == "" {
			return nil, s.prReviewBackendRequiredErr()
		}
	}
	r, ok := s.prReviewRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "pr_reviews")
	}
	return r.ListPRReviews(ctx, prNumber)
}

func (s *Service) ListPRComments(ctx context.Context, backend string, prNumber int) ([]domain.PRComment, error) {
	if backend == "" {
		backend = s.inferPRReviewBackend()
		if backend == "" {
			return nil, s.prReviewBackendRequiredErr()
		}
	}
	r, ok := s.prReviewRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "pr_reviews")
	}
	return r.ListPRComments(ctx, prNumber)
}

func (s *Service) inferPRReviewBackend() string {
	if len(s.prReviewRepos) == 1 {
		for name := range s.prReviewRepos {
			return name
		}
	}
	return ""
}

func (s *Service) prReviewBackendRequiredErr() error {
	if len(s.prReviewRepos) == 0 {
		return fmt.Errorf("%w: no PR-capable backends are configured — add a GitHub or GitLab backend first", ErrBackendRequired)
	}
	names := make([]string, 0, len(s.prReviewRepos))
	for name := range s.prReviewRepos {
		names = append(names, "backend="+name)
	}
	return fmt.Errorf("%w: use %s to query pull request reviews", ErrBackendRequired, strings.Join(names, " or "))
}

// --- Gist operations ---

func (s *Service) CreateGist(ctx context.Context, backend, filename, content string, public bool) (id, url string, err error) {
	r, ok := s.gistRepos[backend]
	if !ok {
		return "", "", s.notSupportedErr(backend, "gists")
	}
	return r.CreateGist(ctx, filename, content, public)
}

func (s *Service) UpdateGist(ctx context.Context, backend, gistID, filename, content string) (string, error) {
	r, ok := s.gistRepos[backend]
	if !ok {
		return "", s.notSupportedErr(backend, "gists")
	}
	return r.UpdateGist(ctx, gistID, filename, content)
}

// --- Dashboard operations (Report Portal) ---

func (s *Service) ListDashboards(ctx context.Context, backend string) ([]domain.Dashboard, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.ListDashboards(ctx)
}

func (s *Service) GetDashboard(ctx context.Context, backend, id string) (*domain.Dashboard, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.GetDashboard(ctx, id)
}

func (s *Service) CreateDashboard(ctx context.Context, backend string, input domain.DashboardCreateInput) (*domain.Dashboard, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.CreateDashboard(ctx, input)
}

func (s *Service) AddWidget(ctx context.Context, backend, dashboardID string, input domain.WidgetAddInput) (*domain.Widget, error) {
	r, ok := s.launchRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "launches")
	}
	return r.AddWidget(ctx, dashboardID, input)
}

// LaunchItemTree builds the item hierarchy tree for a cached launch.
// If the launch is not in cache, it is pulled automatically.
func (s *Service) LaunchItemTree(ctx context.Context, ref string) ([]*domain.ItemTreeNode, error) {
	cacheRef := "reportportal:" + ref
	if _, err := s.launchView.Get(cacheRef); err != nil {
		if _, pullErr := s.ViewPull(ctx, cacheRef); pullErr != nil {
			return nil, pullErr
		}
	}
	return s.launchView.BuildTree(cacheRef)
}

// --- Issue link operations ---

func (s *Service) LinkIssue(ctx context.Context, backend string, input domain.IssueLinkInput) error {
	r, ok := s.issueLinkRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "issue_links")
	}
	return r.CreateIssueLink(ctx, input)
}

func (s *Service) UnlinkIssue(ctx context.Context, backend, inwardKey, outwardKey, linkType string) error {
	r, ok := s.issueLinkRepos[backend]
	if !ok {
		return s.notSupportedErr(backend, "issue_links")
	}
	return r.DeleteIssueLink(ctx, inwardKey, outwardKey, linkType)
}

func (s *Service) ListLinkTypes(ctx context.Context, backend string) ([]domain.IssueLinkType, error) {
	r, ok := s.issueLinkRepos[backend]
	if !ok {
		return nil, s.notSupportedErr(backend, "issue_links")
	}
	return r.ListLinkTypes(ctx)
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

func (s *Service) StagePatch(id string, input domain.StagePatchInput) (*domain.StagedItem, error) {
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

// --- View operations (Local Materialized View) ---

// ViewPull fetches an entity by ref and caches it locally.
// For issue refs (jira:KEY, github:owner/repo#N) — pulls into issue ViewStore.
// For launch refs (reportportal:ID) — pulls launch + all items into LaunchViewStore.
func (s *Service) ViewPull(ctx context.Context, ref string) (any, error) {
	if backend, id, ok := splitLaunchRef(ref); ok {
		return s.launchView.Pull(ctx, backend, id, s.launchRepos)
	}
	return s.view.Pull(ctx, s, ref)
}

// ViewGet returns a cached entity without hitting the backend.
// For launch refs, if the cached view is stale and the launch is non-terminal,
// it re-pulls automatically before returning.
func (s *Service) ViewGet(ctx context.Context, ref string) (any, error) {
	if _, _, ok := splitLaunchRef(ref); ok {
		lv, err := s.launchView.Get(ref)
		if errors.Is(err, ErrStaleView) {
			return s.ViewPull(ctx, ref)
		}
		return lv, err
	}
	return s.view.Get(ref)
}

func (s *Service) ViewMutate(ref, field, value string) error {
	return s.view.Mutate(ref, field, value)
}

func (s *Service) ViewDiff(ref string) (*domain.ViewDiff, error) {
	return s.view.Diff(ref)
}

func (s *Service) ViewPush(ctx context.Context, ref string) (*domain.Issue, error) {
	return s.view.Push(ctx, s, ref)
}

func (s *Service) ViewPushAll(ctx context.Context) ([]string, []string) {
	return s.view.PushAll(ctx, s)
}

type ViewListResult struct {
	Issues   []domain.ViewRecord        `json:"issues"`
	Launches []domain.LaunchViewSummary `json:"launches"`
}

func (s *Service) ViewList() any {
	return ViewListResult{
		Issues:   s.view.List(),
		Launches: s.launchView.List(),
	}
}

func (s *Service) ViewDirty() []*domain.ChangeSet {
	return s.view.Dirty()
}

func (s *Service) ViewDrop(ref string) {
	if _, _, ok := splitLaunchRef(ref); ok {
		s.launchView.Drop(ref)
		return
	}
	s.view.Drop(ref)
}

func (s *Service) ViewReset() {
	s.view.Reset()
	s.launchView.Reset()
}

// splitLaunchRef checks if ref is a reportportal launch ref (e.g. "reportportal:37337").
func splitLaunchRef(ref string) (backend, id string, ok bool) {
	const rpPrefix = "reportportal:"
	if !strings.HasPrefix(ref, rpPrefix) {
		return "", "", false
	}
	return "reportportal", ref[len(rpPrefix):], true
}

// --- Service options ---

// ServiceOption configures optional dependencies on the Service.
type ServiceOption func(*Service)

// WithLinkExtractor injects a LinkExtractor for triage.
func WithLinkExtractor(e repository.LinkExtractor) ServiceOption {
	return func(s *Service) { s.extractor = e }
}

// WithGraphStore injects a GraphStore for triage.
func WithGraphStore(g repository.GraphStore) ServiceOption {
	return func(s *Service) { s.graphStore = g }
}

// WithLedger injects a Ledger for artifact record tracking.
func WithLedger(l repository.Ledger) ServiceOption {
	return func(s *Service) { s.ledger = l }
}

// WithCursor injects a Cursor for delta sync pollers.
func WithCursor(c poller.Cursor) ServiceOption {
	return func(s *Service) { s.cursor = c }
}

// WithWatchScopes sets the per-backend delta sync scopes.
// Only backends with a non-empty WatchScope get a delta sync poller.
func WithWatchScopes(scopes map[string]domain.WatchScope) ServiceOption {
	return func(s *Service) { s.watchScopes = scopes }
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
func (s *Service) GetTriageConfig() service.TriageConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return service.TriageConfig{
		RateLimit: s.crawlRateLimit,
		AllowList: s.crawlAllowList,
	}
}

// SetTriageConfig updates triage crawl settings at runtime.
func (s *Service) SetTriageConfig(cfg service.TriageConfig) {
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

// --- Delta sync pollers ---

// BuildPollers registers delta sync pollers for backends that have a non-empty
// WatchScope configured. Nothing is registered for backends without a watch: block.
// Field manifest pollers are registered separately in each backend's init().
func (s *Service) BuildPollers() {
	cur := s.cursor
	if cur == nil {
		cur = poller.NewNopCursor()
	}

	s.mu.RLock()
	launchRepos := make(map[string]repository.LaunchRepository, len(s.launchRepos))
	for k, v := range s.launchRepos {
		launchRepos[k] = v
	}
	deltaRepos := make(map[string]repository.DeltaSyncer, len(s.deltaRepos))
	for k, v := range s.deltaRepos {
		deltaRepos[k] = v
	}
	scopes := make(map[string]domain.WatchScope, len(s.watchScopes))
	for k, v := range s.watchScopes {
		scopes[k] = v
	}
	s.mu.RUnlock()

	for name, repo := range launchRepos {
		scope, ok := scopes[name]
		if !ok || scope.IsEmpty() {
			continue
		}
		name, repo, scope := name, repo, scope
		poller.Register("launches:"+name, poller.New(
			"launches:"+name,
			func() bool { return true },
			func(ctx context.Context) error { return s.syncLaunches(ctx, name, repo, scope, cur) },
		))
	}

	for name, repo := range deltaRepos {
		scope, ok := scopes[name]
		if !ok || scope.IsEmpty() {
			continue
		}
		name, repo, scope := name, repo, scope
		poller.Register("delta:"+name, poller.New(
			"delta:"+name,
			func() bool { return true },
			func(ctx context.Context) error { return s.syncIssues(ctx, name, repo, scope, cur) },
		))
	}
}

// syncLaunches fetches launches created after the cursor that match scope,
// auto-pulls those matching configured statuses, and advances the cursor.
func (s *Service) syncLaunches(ctx context.Context, backend string, repo repository.LaunchRepository, scope domain.WatchScope, cur poller.Cursor) error {
	since := cur.Get("launches:" + backend)
	launches, err := repo.ListLaunches(ctx, domain.LaunchFilter{StartAfter: since, Limit: 50})
	if err != nil {
		return fmt.Errorf("launch delta sync %s: %w", backend, err)
	}

	wantStatus := make(map[string]bool, len(scope.Statuses))
	for _, s := range scope.Statuses {
		wantStatus[s] = true
	}

	var newest time.Time
	for _, launch := range launches {
		if launch.StartTime.After(newest) {
			newest = launch.StartTime
		}
		if !matchesLaunchScope(launch, scope, wantStatus) {
			continue
		}
		_, _ = s.launchView.Pull(ctx, backend, launch.ID, s.launchRepos)
	}
	if !newest.IsZero() && newest.After(since) {
		return cur.Set("launches:"+backend, newest)
	}
	return nil
}

// matchesLaunchScope reports whether a launch satisfies the watch scope filters.
func matchesLaunchScope(launch domain.Launch, scope domain.WatchScope, wantStatus map[string]bool) bool {
	if len(wantStatus) > 0 && !wantStatus[launch.Status] {
		return false
	}
	if len(scope.NamePatterns) > 0 {
		matched := false
		for _, pat := range scope.NamePatterns {
			if strings.Contains(launch.Name, pat) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// syncIssues fetches issues updated after the cursor via DeltaSyncer,
// upserts them into the ledger, and advances the cursor.
func (s *Service) syncIssues(ctx context.Context, backend string, repo repository.DeltaSyncer, scope domain.WatchScope, cur poller.Cursor) error {
	if s.ledger == nil {
		return nil
	}
	since := cur.Get("delta:" + backend)
	issues, err := repo.ListUpdatedSince(ctx, since, scope, 50)
	if err != nil {
		return fmt.Errorf("issue delta sync %s: %w", backend, err)
	}
	var newest time.Time
	for i := range issues {
		if issues[i].UpdatedAt.After(newest) {
			newest = issues[i].UpdatedAt
		}
		_ = s.ledger.Put(ctx, issueToRecord(backend, &issues[i]))
	}
	if !newest.IsZero() && newest.After(since) {
		return cur.Set("delta:"+backend, newest)
	}
	return nil
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
