package domain

import (
	"testing"
)

type templateTestCase struct {
	name     string
	descs    []string
	want     []string
	wantBody string
}

func templateTestCases() []templateTestCase { //nolint:funlen
	return []templateTestCase{
		{
			name: "OCPBUGS Bug template — all sections present in every sample",
			descs: []string{
				"Description of problem: VFs not configured\n\nVersion-Release number of selected component (if applicable): 4.22\n\nHow reproducible: 100%\n\nSteps to Reproduce:\n1. Create an altname\n2. Create policy\n\nActual results: Node enters boot loop\n\nExpected results: VFs configured\n\nAdditional info: none",
				"Description of problem: PTP DPLL issue\n\nVersion-Release number of selected component (if applicable): 4.22\n\nHow reproducible: intermittent\n\nSteps to Reproduce:\n1. Deploy T-BC\n2. Update PtpConfig\n\nActual results: FREERUN\n\nExpected results: HOLDOVER\n\nAdditional info:",
			},
			want: []string{
				"Description of problem:",
				"Version-Release number of selected component (if applicable):",
				"How reproducible:",
				"Steps to Reproduce:",
				"Actual results:",
				"Expected results:",
				"Additional info:",
			},
			wantBody: "Description of problem:\n\nVersion-Release number of selected component (if applicable):\n\nHow reproducible:\n\nSteps to Reproduce:\n\nActual results:\n\nExpected results:\n\nAdditional info:",
		},
		{
			name: "partial overlap — only common sections survive",
			descs: []string{
				"Summary:\n\nSteps:\n1. Do X\n\nResult:\n\nNotes:",
				"Summary:\n\nSteps:\n1. Do Y\n\nResult:",
			},
			want: []string{
				"Summary:",
				"Steps:",
				"Result:",
			},
			wantBody: "Summary:\n\nSteps:\n\nResult:",
		},
		{
			name: "single sample",
			descs: []string{
				"Problem:\n\nImpact:\n\nWorkaround:",
			},
			want: []string{
				"Problem:",
				"Impact:",
				"Workaround:",
			},
			wantBody: "Problem:\n\nImpact:\n\nWorkaround:",
		},
		{
			name:  "empty input",
			descs: []string{},
			want:  nil,
		},
		{
			name:  "all empty descriptions",
			descs: []string{"", "  ", "\n"},
			want:  nil,
		},
		{
			name: "no common sections",
			descs: []string{
				"This is a plain text bug report with no sections.",
				"Another unstructured report about a different thing.",
			},
			want: nil,
		},
	}
}

func TestExtractTemplateSections(t *testing.T) {
	for _, tt := range templateTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTemplateSections(tt.descs)
			if tt.want == nil {
				if got != nil {
					t.Errorf("want nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("section count: got %d, want %d\n  got:  %v\n  want: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("section[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildTemplateBody(t *testing.T) {
	for _, tt := range templateTestCases() {
		if tt.wantBody == "" {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			sections := ExtractTemplateSections(tt.descs)
			body := BuildTemplateBody(sections)
			if body != tt.wantBody {
				t.Errorf("body mismatch:\n  got:  %q\n  want: %q", body, tt.wantBody)
			}
		})
	}
}

func TestExtractTemplateSections_IgnoresCodeBlocks(t *testing.T) {
	descs := []string{
		"{code:java}Description of problem:\n\nVersion-Release number of selected component (if applicable):\n\nHow reproducible:\n\nSteps to Reproduce:\n\nActual results:\n\nExpected results:\n\nAdditional info:{code}",
		"{code:none}\nDescription of problem:\n\nVersion-Release number of selected component (if applicable):\n\nHow reproducible:\n\nSteps to Reproduce:\n\nActual results:\n\nExpected results:\n\nAdditional info:\n{code}",
	}
	got := ExtractTemplateSections(descs)
	want := []string{
		"Description of problem:",
		"Version-Release number of selected component (if applicable):",
		"How reproducible:",
		"Steps to Reproduce:",
		"Actual results:",
		"Expected results:",
		"Additional info:",
	}
	if len(got) != len(want) {
		t.Fatalf("section count: got %d, want %d\n  got:  %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
