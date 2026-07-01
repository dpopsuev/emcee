// Package scribe translates Emcee domain types into Battery canonical Records.
package scribe

import (
	"strings"

	"github.com/dpopsuev/battery/translate"
	"github.com/dpopsuev/emcee/internal/domain"
)

// TranslateIssues converts Emcee issues into canonical Records.
func TranslateIssues(issues []domain.Issue) translate.Result {
	var result translate.Result

	for _, issue := range issues {
		kind := issueKind(issue)
		labels := []string{"source:emcee"}
		if issue.Project != "" {
			labels = append(labels, "project:"+slugify(issue.Project))
		}
		labels = append(labels, issue.Labels...)

		var sections []translate.Section
		if len(issue.Components) > 0 {
			sections = append(sections, translate.Section{
				Name: "components",
				Text: strings.Join(issue.Components, ", "),
			})
		}

		r := translate.Record{
			ID:       issue.Ref,
			Kind:     kind,
			Title:    issue.Title,
			Labels:   labels,
			Sections: sections,
			Extra: map[string]any{
				"ref_backend": "emcee",
				"ref_id":      issue.Ref,
				"status":      string(issue.Status),
				"raw_status":  issue.RawStatus,
				"substatus":   issue.Substatus,
				"priority":    issue.Priority.String(),
				"assignee":    issue.Assignee,
				"issue_type":  issue.IssueType,
			},
		}
		result.Records = append(result.Records, r)
	}

	return result
}

// TranslateTriageGraph converts a triage result into Records + Edges.
func TranslateTriageGraph(graph *domain.TriageGraph) translate.Result {
	var result translate.Result

	for _, node := range graph.Nodes {
		r := translate.Record{
			ID:     node.Ref,
			Kind:   triageNodeKind(node.Type),
			Title:  node.Title,
			Labels: []string{"source:emcee", "triage:" + graph.Seed},
			Extra: map[string]any{
				"phase":  node.Phase,
				"status": node.Status,
			},
		}
		result.Records = append(result.Records, r)
	}

	for _, edge := range graph.Edges {
		result.Edges = append(result.Edges, translate.Edge{
			From:     edge.From,
			Relation: edge.Type,
			To:       edge.To,
		})
	}

	return result
}

func issueKind(issue domain.Issue) string {
	switch strings.ToLower(issue.IssueType) {
	case "bug":
		return "intent.bug"
	case "story", "feature":
		return "intent.spec"
	default:
		return "effort.task"
	}
}

func triageNodeKind(nodeType string) string {
	switch nodeType {
	case "issue":
		return "intent.bug"
	case "pr":
		return "support.ref"
	case "build", "launch":
		return "support.ref"
	default:
		return "knowledge.note"
	}
}

func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}

