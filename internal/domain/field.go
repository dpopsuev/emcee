package domain

// Field represents a metadata field available on a backend.
type Field struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
	Schema string `json:"schema,omitempty"`
}
