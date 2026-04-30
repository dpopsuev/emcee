package domain

// Section represents a heading-delimited section of a markdown document.
type Section struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Level     int       `json:"level"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Children  []Section `json:"children,omitempty"`
}

// DocLink represents an internal or external link found in a document.
type DocLink struct {
	Text        string `json:"text"`
	Destination string `json:"destination"`
	Line        int    `json:"line"`
	SectionID   string `json:"section_id,omitempty"`
	Dead        bool   `json:"dead,omitempty"`
}

// CodeBlock represents a fenced code block in a document.
type CodeBlock struct {
	Language  string `json:"language"`
	Content   string `json:"content"`
	Line      int    `json:"line"`
	SectionID string `json:"section_id,omitempty"`
}

// DocumentTree is the parsed structural representation of a markdown document.
type DocumentTree struct {
	Title      string      `json:"title"`
	Sections   []Section   `json:"sections"`
	Links      []DocLink   `json:"links,omitempty"`
	CodeBlocks []CodeBlock `json:"code_blocks,omitempty"`
	LineCount  int         `json:"line_count"`
}
