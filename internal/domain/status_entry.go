package domain

// StatusEntry represents a backend status and its category.
// Returned by StatusRepository.ListStatuses for manifest discovery.
type StatusEntry struct {
	Name        string `json:"name"`
	CategoryKey string `json:"category_key"`
}
