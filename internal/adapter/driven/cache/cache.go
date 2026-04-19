// Package cache provides a CachingRepository decorator that wraps any
// driven.IssueRepository with an LRU+TTL read cache. Write operations
// delegate to the inner repository and invalidate affected entries.
package cache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var (
	// ErrCommentsNotSupported indicates the inner repository does not implement comments.
	ErrCommentsNotSupported = errors.New("comments not supported")
	// ErrNotSupported indicates the inner repository does not implement the requested interface.
	ErrNotSupported = errors.New("not supported")
)

// Default configuration.
const (
	DefaultGetTTL      = 15 * time.Minute
	DefaultListTTL     = 5 * time.Minute
	DefaultSearchTTL   = 5 * time.Minute
	DefaultChildrenTTL = 10 * time.Minute
	DefaultCommentsTTL = 5 * time.Minute
	DefaultCapacity    = 256
)

var _ driven.IssueRepository = (*Repository)(nil)

type entry struct {
	key       string
	value     any
	expiresAt time.Time
}

// Repository wraps an IssueRepository with an LRU+TTL read cache.
// Optional interfaces (CommentRepository, LaunchRepository, etc.) are
// detected on the inner repo and passed through transparently.
type Repository struct {
	inner       driven.IssueRepository
	comments    driven.CommentRepository
	launches    driven.LaunchRepository
	fields      driven.FieldRepository
	jql         driven.JQLRepository
	docs        driven.DocumentRepository
	projects    driven.ProjectRepository
	initiatives driven.InitiativeRepository
	labels      driven.LabelRepository
	bulk        driven.BulkIssueRepository
	prs         driven.PRRepository
	builds      driven.BuildRepository
	caps        []string

	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	capacity int

	getTTL      time.Duration
	listTTL     time.Duration
	searchTTL   time.Duration
	childrenTTL time.Duration
	commentsTTL time.Duration
	now         func() time.Time
}

// New wraps an IssueRepository with caching.
func New(inner driven.IssueRepository, opts ...Option) *Repository {
	r := &Repository{
		inner:       inner,
		items:       make(map[string]*list.Element),
		order:       list.New(),
		capacity:    DefaultCapacity,
		getTTL:      DefaultGetTTL,
		listTTL:     DefaultListTTL,
		searchTTL:   DefaultSearchTTL,
		childrenTTL: DefaultChildrenTTL,
		commentsTTL: DefaultCommentsTTL,
		now:         time.Now,
	}
	if v, ok := inner.(driven.CommentRepository); ok {
		r.comments = v
		r.caps = append(r.caps, "comments")
	}
	if v, ok := inner.(driven.LaunchRepository); ok {
		r.launches = v
		r.caps = append(r.caps, "launches")
	}
	if v, ok := inner.(driven.FieldRepository); ok {
		r.fields = v
		r.caps = append(r.caps, "fields")
	}
	if v, ok := inner.(driven.JQLRepository); ok {
		r.jql = v
		r.caps = append(r.caps, "jql")
	}
	if v, ok := inner.(driven.DocumentRepository); ok {
		r.docs = v
		r.caps = append(r.caps, "documents")
	}
	if v, ok := inner.(driven.ProjectRepository); ok {
		r.projects = v
		r.caps = append(r.caps, "projects")
	}
	if v, ok := inner.(driven.InitiativeRepository); ok {
		r.initiatives = v
		r.caps = append(r.caps, "initiatives")
	}
	if v, ok := inner.(driven.LabelRepository); ok {
		r.labels = v
		r.caps = append(r.caps, "labels")
	}
	if v, ok := inner.(driven.BulkIssueRepository); ok {
		r.bulk = v
		r.caps = append(r.caps, "bulk")
	}
	if v, ok := inner.(driven.PRRepository); ok {
		r.prs = v
		r.caps = append(r.caps, "prs")
	}
	if v, ok := inner.(driven.BuildRepository); ok {
		r.builds = v
		r.caps = append(r.caps, "builds")
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Option configures the cache.
type Option func(*Repository)

func WithCapacity(n int) Option          { return func(r *Repository) { r.capacity = n } }
func WithGetTTL(d time.Duration) Option  { return func(r *Repository) { r.getTTL = d } }
func WithListTTL(d time.Duration) Option { return func(r *Repository) { r.listTTL = d } }

// WithNow injects a clock function for testing.
func WithNow(fn func() time.Time) Option { return func(r *Repository) { r.now = fn } }

// --- Pass-through ---

func (r *Repository) Name() string { return r.inner.Name() }

// Capabilities returns the list of optional interfaces the inner repo supports.
func (r *Repository) Capabilities() []string { return r.caps }

// --- Cached reads ---

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	start := r.now()
	ck := "get:" + key
	if v, ok := r.cacheGet(ck); ok {
		r.logRead(ctx, "get", true, r.now().Sub(start))
		return v.(*domain.Issue), nil
	}
	issue, err := r.inner.Get(ctx, key)
	elapsed := r.now().Sub(start)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issue, r.getTTL)
	r.logRead(ctx, "get", false, elapsed)
	return issue, nil
}

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	start := r.now()
	ck := "list:" + hashJSON(filter)
	if v, ok := r.cacheGet(ck); ok {
		r.logRead(ctx, "list", true, r.now().Sub(start))
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.List(ctx, filter)
	elapsed := r.now().Sub(start)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.listTTL)
	r.logRead(ctx, "list", false, elapsed)
	return issues, nil
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.Issue, error) {
	start := r.now()
	ck := fmt.Sprintf("search:%s:%d", query, limit)
	if v, ok := r.cacheGet(ck); ok {
		r.logRead(ctx, "search", true, r.now().Sub(start))
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.Search(ctx, query, limit)
	elapsed := r.now().Sub(start)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.searchTTL)
	r.logRead(ctx, "search", false, elapsed)
	return issues, nil
}

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	start := r.now()
	ck := "children:" + key
	if v, ok := r.cacheGet(ck); ok {
		r.logRead(ctx, "list_children", true, r.now().Sub(start))
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.ListChildren(ctx, key)
	elapsed := r.now().Sub(start)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.childrenTTL)
	r.logRead(ctx, "list_children", false, elapsed)
	return issues, nil
}

// --- Write-through (invalidate on success) ---

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	issue, err := r.inner.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	r.invalidatePrefix("list:")
	r.invalidatePrefix("search:")
	return issue, nil
}

func (r *Repository) Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	issue, err := r.inner.Update(ctx, key, input)
	if err != nil {
		return nil, err
	}
	r.invalidate("get:" + key)
	r.invalidatePrefix("list:")
	r.invalidatePrefix("search:")
	r.invalidatePrefix("children:")
	return issue, nil
}

// --- Comment operations (conditional) ---

func (r *Repository) ListComments(ctx context.Context, key string) ([]domain.Comment, error) {
	if r.comments == nil {
		return nil, fmt.Errorf("%w by %s", ErrCommentsNotSupported, r.inner.Name())
	}
	start := r.now()
	ck := "comments:" + key
	if v, ok := r.cacheGet(ck); ok {
		r.logRead(ctx, "list_comments", true, r.now().Sub(start))
		return v.([]domain.Comment), nil
	}
	comments, err := r.comments.ListComments(ctx, key)
	elapsed := r.now().Sub(start)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, comments, r.commentsTTL)
	r.logRead(ctx, "list_comments", false, elapsed)
	return comments, nil
}

func (r *Repository) AddComment(ctx context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error) {
	if r.comments == nil {
		return nil, fmt.Errorf("%w by %s", ErrCommentsNotSupported, r.inner.Name())
	}
	comment, err := r.comments.AddComment(ctx, key, input)
	if err != nil {
		return nil, err
	}
	r.invalidate("comments:" + key)
	return comment, nil
}

// --- Internal cache operations ---

func (r *Repository) cacheGet(key string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	el, ok := r.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*entry)
	if r.now().After(e.expiresAt) {
		r.removeLocked(el, key)
		return nil, false
	}
	r.order.MoveToFront(el)
	return e.value, true
}

func (r *Repository) cachePut(key string, value any, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if el, ok := r.items[key]; ok {
		r.order.MoveToFront(el)
		e := el.Value.(*entry)
		e.value = value
		e.expiresAt = r.now().Add(ttl)
		return
	}
	for r.order.Len() >= r.capacity {
		back := r.order.Back()
		if back == nil {
			break
		}
		e := back.Value.(*entry)
		r.removeLocked(back, e.key)
	}
	e := &entry{key: key, value: value, expiresAt: r.now().Add(ttl)}
	el := r.order.PushFront(e)
	r.items[key] = el
}

func (r *Repository) invalidate(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if el, ok := r.items[key]; ok {
		r.removeLocked(el, key)
	}
}

func (r *Repository) invalidatePrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, el := range r.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			r.removeLocked(el, k)
		}
	}
}

func (r *Repository) removeLocked(el *list.Element, key string) {
	r.order.Remove(el)
	delete(r.items, key)
}

const (
	logMsgCacheRead = "cache read"
	logKeyBackend   = "backend"
	logKeyOp        = "op"
	logKeyCacheHit  = "cache_hit"
	logKeyElapsed   = "elapsed"
)

func (r *Repository) logRead(ctx context.Context, op string, cacheHit bool, elapsed time.Duration) {
	slog.LogAttrs(ctx, slog.LevelDebug, logMsgCacheRead,
		slog.String(logKeyBackend, r.inner.Name()),
		slog.String(logKeyOp, op),
		slog.Bool(logKeyCacheHit, cacheHit),
		slog.Duration(logKeyElapsed, elapsed),
	)
}

func hashJSON(v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

// --- Passthrough: LaunchRepository ---

func (r *Repository) ListLaunches(ctx context.Context, filter domain.LaunchFilter) ([]domain.Launch, error) {
	if r.launches == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.launches.ListLaunches(ctx, filter)
}

func (r *Repository) GetLaunch(ctx context.Context, id string) (*domain.Launch, error) {
	if r.launches == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.launches.GetLaunch(ctx, id)
}

func (r *Repository) ListTestItems(ctx context.Context, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	if r.launches == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.launches.ListTestItems(ctx, launchID, filter)
}

func (r *Repository) GetTestItem(ctx context.Context, id string) (*domain.TestItem, error) {
	if r.launches == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.launches.GetTestItem(ctx, id)
}

func (r *Repository) UpdateDefects(ctx context.Context, updates []domain.DefectUpdate) error {
	if r.launches == nil {
		return fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.launches.UpdateDefects(ctx, updates)
}

// --- Passthrough: FieldRepository ---

func (r *Repository) ListFields(ctx context.Context) ([]domain.Field, error) {
	if r.fields == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.fields.ListFields(ctx)
}

// --- Passthrough: JQLRepository ---

func (r *Repository) SearchJQL(ctx context.Context, jql string, limit int) ([]domain.Issue, error) {
	if r.jql == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.jql.SearchJQL(ctx, jql, limit)
}

// --- Passthrough: DocumentRepository ---

func (r *Repository) ListDocuments(ctx context.Context, filter domain.DocumentListFilter) ([]domain.Document, error) {
	if r.docs == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.docs.ListDocuments(ctx, filter)
}

func (r *Repository) CreateDocument(ctx context.Context, input domain.DocumentCreateInput) (*domain.Document, error) {
	if r.docs == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.docs.CreateDocument(ctx, input)
}

// --- Passthrough: ProjectRepository ---

func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	if r.projects == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.projects.ListProjects(ctx, filter)
}

func (r *Repository) CreateProject(ctx context.Context, input domain.ProjectCreateInput) (*domain.Project, error) {
	if r.projects == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.projects.CreateProject(ctx, input)
}

func (r *Repository) UpdateProject(ctx context.Context, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	if r.projects == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.projects.UpdateProject(ctx, id, input)
}

// --- Passthrough: InitiativeRepository ---

func (r *Repository) ListInitiatives(ctx context.Context, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	if r.initiatives == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.initiatives.ListInitiatives(ctx, filter)
}

func (r *Repository) CreateInitiative(ctx context.Context, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	if r.initiatives == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.initiatives.CreateInitiative(ctx, input)
}

// --- Passthrough: LabelRepository ---

func (r *Repository) ListLabels(ctx context.Context) ([]domain.Label, error) {
	if r.labels == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.labels.ListLabels(ctx)
}

func (r *Repository) CreateLabel(ctx context.Context, input domain.LabelCreateInput) (*domain.Label, error) {
	if r.labels == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.labels.CreateLabel(ctx, input)
}

// --- Passthrough: BulkIssueRepository ---

func (r *Repository) BulkCreateIssues(ctx context.Context, inputs []domain.CreateInput) ([]domain.Issue, error) {
	if r.bulk == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.bulk.BulkCreateIssues(ctx, inputs)
}

// --- Passthrough: PRRepository ---

func (r *Repository) ListPRs(ctx context.Context, filter domain.PRFilter) ([]domain.PullRequest, error) {
	if r.prs == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.prs.ListPRs(ctx, filter)
}

// --- Passthrough: BuildRepository ---

func (r *Repository) ListJobs(ctx context.Context, filter domain.JobFilter) ([]domain.Job, error) {
	if r.builds == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.ListJobs(ctx, filter)
}

func (r *Repository) GetJob(ctx context.Context, name string) (*domain.Job, error) {
	if r.builds == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.GetJob(ctx, name)
}

func (r *Repository) TriggerBuild(ctx context.Context, jobName string, params map[string]string) (int64, error) {
	if r.builds == nil {
		return 0, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.TriggerBuild(ctx, jobName, params)
}

func (r *Repository) GetBuild(ctx context.Context, jobName string, number int64) (*domain.Build, error) {
	if r.builds == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.GetBuild(ctx, jobName, number)
}

func (r *Repository) GetBuildLog(ctx context.Context, jobName string, number int64) (string, error) {
	if r.builds == nil {
		return "", fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.GetBuildLog(ctx, jobName, number)
}

func (r *Repository) GetTestResults(ctx context.Context, jobName string, number int64) (*domain.TestResult, error) {
	if r.builds == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.GetTestResults(ctx, jobName, number)
}

func (r *Repository) GetQueue(ctx context.Context) ([]domain.QueueItem, error) {
	if r.builds == nil {
		return nil, fmt.Errorf("%w by %s", ErrNotSupported, r.inner.Name())
	}
	return r.builds.GetQueue(ctx)
}
