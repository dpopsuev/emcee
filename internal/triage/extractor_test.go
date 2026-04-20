package triage

import (
	"context"
	"testing"
)

func TestRegexLinkExtractor_JiraKey(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "Fixed in OCPBUGS-12345 and related to TELCOSTRAT-85")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := map[string]bool{"jira:OCPBUGS-12345": true, "jira:TELCOSTRAT-85": true}
	got := make(map[string]bool)
	for _, r := range refs {
		got[r.Ref] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing ref %q", k)
		}
	}
}

func TestRegexLinkExtractor_GitHubPR(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "See https://github.com/openshift/ptp-operator/pull/234 for details")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 1 || refs[0].Ref != "github:openshift/ptp-operator#234" {
		t.Errorf("got %v, want [github:openshift/ptp-operator#234]", refs)
	}
}

func TestRegexLinkExtractor_GitLabMR(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "Fixed in https://gitlab.cee.redhat.com/ocp-edge-qe/ptp-daemon/-/merge_requests/789")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 1 || refs[0].Ref != "gitlab:ocp-edge-qe/ptp-daemon!789" {
		t.Errorf("got %v, want [gitlab:ocp-edge-qe/ptp-daemon!789]", refs)
	}
}

func TestRegexLinkExtractor_JenkinsBuild(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "Build failed: https://jenkins-csb-kniqe-auto.dno.corp.redhat.com/job/ocp-far-edge-vran-deployment/8892")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 1 || refs[0].Ref != "jenkins:ocp-far-edge-vran-deployment#8892" {
		t.Errorf("got %v, want [jenkins:ocp-far-edge-vran-deployment#8892]", refs)
	}
}

func TestRegexLinkExtractor_RPLaunch(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "Results at https://reportportal.example.com/ui/#project/launches/all/4567")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 1 || refs[0].Ref != "reportportal:launch/4567" {
		t.Errorf("got %v, want [reportportal:launch/4567]", refs)
	}
}

func TestRegexLinkExtractor_MultiplePatterns(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	text := `Bug OCPBUGS-999 was detected in https://jenkins-csb-kniqe-ci.dno.corp.redhat.com/job/my-pipeline/42
Fixed by https://github.com/openshift/ptp-operator/pull/100
See launches/all/555 for test results`

	refs, err := e.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 4 {
		t.Errorf("got %d refs, want 4", len(refs))
		for _, r := range refs {
			t.Logf("  %s (%s)", r.Ref, r.Source)
		}
	}
}

func TestRegexLinkExtractor_Deduplication(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "OCPBUGS-123 is related to OCPBUGS-123 (duplicate mention)")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("got %d refs, want 1 (deduped)", len(refs))
	}
}

func TestRegexLinkExtractor_NoMatches(t *testing.T) {
	e := NewRegexLinkExtractor(nil)
	refs, err := e.Extract(context.Background(), "This text has no cross-references at all.")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("got %d refs, want 0", len(refs))
	}
}
