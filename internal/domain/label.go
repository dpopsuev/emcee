package domain

// Label is a tag that can be applied to issues.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type LabelCreateInput struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}
