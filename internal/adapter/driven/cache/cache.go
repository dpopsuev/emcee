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
	"sync"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// ErrCommentsNotSupported indicates the inner repository does not implement comments.
var ErrCommentsNotSupported = errors.New("comments not supported")

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
// If the inner repository implements CommentRepository, the wrapper
// transparently provides cached comment access too.
type Repository struct {
	inner    driven.IssueRepository
	comments driven.CommentRepository

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
	if cr, ok := inner.(driven.CommentRepository); ok {
		r.comments = cr
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

// --- Cached reads ---

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	ck := "get:" + key
	if v, ok := r.cacheGet(ck); ok {
		return v.(*domain.Issue), nil
	}
	issue, err := r.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issue, r.getTTL)
	return issue, nil
}

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	ck := "list:" + hashJSON(filter)
	if v, ok := r.cacheGet(ck); ok {
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.listTTL)
	return issues, nil
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.Issue, error) {
	ck := fmt.Sprintf("search:%s:%d", query, limit)
	if v, ok := r.cacheGet(ck); ok {
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.searchTTL)
	return issues, nil
}

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	ck := "children:" + key
	if v, ok := r.cacheGet(ck); ok {
		return v.([]domain.Issue), nil
	}
	issues, err := r.inner.ListChildren(ctx, key)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, issues, r.childrenTTL)
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
	ck := "comments:" + key
	if v, ok := r.cacheGet(ck); ok {
		return v.([]domain.Comment), nil
	}
	comments, err := r.comments.ListComments(ctx, key)
	if err != nil {
		return nil, err
	}
	r.cachePut(ck, comments, r.commentsTTL)
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

func hashJSON(v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
