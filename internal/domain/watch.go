package domain

// WatchScope controls what a delta sync poller fetches.
// An empty WatchScope (all fields nil/empty) disables the poller — nothing
// is synced unless at least one field is set.
type WatchScope struct {
	// Projects scopes issue sync to specific project keys, owner/repo paths,
	// or team identifiers depending on the backend.
	// Jira: project keys (e.g. OCPBUGS). GitHub: owner/repo. GitLab: namespace/project.
	Projects []string

	// Labels restricts to issues carrying all listed labels.
	Labels []string

	// IssueTypes restricts to specific issue types (e.g. Bug, Story).
	// Jira only; ignored by other backends.
	IssueTypes []string

	// NamePatterns scopes launch sync to names matching any listed substring.
	// Report Portal only.
	NamePatterns []string

	// Statuses restricts launch sync to launches in the listed statuses.
	// Report Portal only. Empty means all statuses.
	Statuses []string
}

// IsEmpty reports whether the scope has no filtering criteria set.
// BuildPollers skips backends with an empty scope.
func (s WatchScope) IsEmpty() bool {
	return len(s.Projects) == 0 &&
		len(s.Labels) == 0 &&
		len(s.IssueTypes) == 0 &&
		len(s.NamePatterns) == 0 &&
		len(s.Statuses) == 0
}
