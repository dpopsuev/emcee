package application_test

import (
	"testing"

	"github.com/dpopsuev/emcee/internal/application"
)

func TestProjectKeyFromRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"jira:PROJ-42", "PROJ"},
		{"jira:ABC-123", "ABC"},
		{"jira:MYTEAM-1", "MYTEAM"},
		{"linear:HEG-17", "HEG"},
		{"PROJ-42", ""},
		{"", ""},
		{"github:owner/repo#42", ""},
		{"jira:AB-CD-42", "AB-CD"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := application.ProjectKeyFromRef(tt.ref)
			if got != tt.want {
				t.Errorf("ProjectKeyFromRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
