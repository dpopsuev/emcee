package domain

// Initiative is a strategic objective that groups projects.
type Initiative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	URL         string `json:"url,omitempty"`
}

type InitiativeCreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type InitiativeListFilter struct {
	Limit int `json:"limit,omitempty"`
}
