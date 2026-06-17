package scribe_test

import (
	"testing"

	bridge "github.com/dpopsuev/emcee/bridges/scribe"
	"github.com/dpopsuev/emcee/testdata"
)

func TestTranslateIssues_Kinds(t *testing.T) {
	issues := testdata.SampleIssues()
	result := bridge.TranslateIssues(issues)

	if len(result.Records) != 3 {
		t.Fatalf("records = %d; want 3", len(result.Records))
	}

	bug := result.Records[0]
	if bug.Kind != "intent.bug" {
		t.Errorf("bug kind = %q; want intent.bug", bug.Kind)
	}
	if bug.ID != "jira:AUTH-42" {
		t.Errorf("bug id = %q; want jira:AUTH-42", bug.ID)
	}

	story := result.Records[1]
	if story.Kind != "intent.spec" {
		t.Errorf("story kind = %q; want intent.spec", story.Kind)
	}
}

func TestTranslateIssues_Labels(t *testing.T) {
	issues := testdata.SampleIssues()
	result := bridge.TranslateIssues(issues)

	bug := result.Records[0]
	hasSource := false
	hasProject := false
	for _, l := range bug.Labels {
		if l == "source:emcee" {
			hasSource = true
		}
		if l == "project:auth-service" {
			hasProject = true
		}
	}
	if !hasSource {
		t.Error("missing source:emcee label")
	}
	if !hasProject {
		t.Error("missing project:auth-service label")
	}
}

func TestTranslateIssues_Extra(t *testing.T) {
	issues := testdata.SampleIssues()
	result := bridge.TranslateIssues(issues)

	bug := result.Records[0]
	if bug.Extra["assignee"] != "alice" {
		t.Errorf("assignee = %v; want alice", bug.Extra["assignee"])
	}
	if bug.Extra["priority"] != "high" {
		t.Errorf("priority = %v; want high", bug.Extra["priority"])
	}
}

func TestTranslateTriageGraph(t *testing.T) {
	graph := testdata.SampleTriageGraph()
	result := bridge.TranslateTriageGraph(graph)

	if len(result.Records) != 3 {
		t.Fatalf("records = %d; want 3", len(result.Records))
	}
	if len(result.Edges) != 2 {
		t.Fatalf("edges = %d; want 2", len(result.Edges))
	}

	edge := result.Edges[0]
	if edge.Relation != "fixed_by" {
		t.Errorf("relation = %q; want fixed_by", edge.Relation)
	}
	if edge.From != "jira:AUTH-42" {
		t.Errorf("from = %q; want jira:AUTH-42", edge.From)
	}
}

func TestTranslateIssues_Empty(t *testing.T) {
	result := bridge.TranslateIssues(nil)
	if len(result.Records) != 0 {
		t.Errorf("records = %d; want 0", len(result.Records))
	}
}
