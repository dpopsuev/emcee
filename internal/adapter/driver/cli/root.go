// Package cli implements the driver (inbound) adapter for the cobra CLI.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/DanyPops/emcee/internal/adapter/driven/linear"
	mcpserver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
	"github.com/DanyPops/emcee/internal/app"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/spf13/cobra"
)

var (
	flagBackend     string
	flagConfig      string
	flagProject     string
	flagStatus      string
	flagPriority    string
	flagLabels      []string
	flagLimit       int
	flagJSON        bool
	flagContent     string
	flagFile        string
	flagParent      string
	flagAssignee    string
	flagProjectID   string
	flagDescription string
	flagColor       string
)

func newService() (*app.Service, error) {
	if config.Exists(flagConfig) {
		return newServiceFromConfig()
	}
	return newServiceFromEnv()
}

func newServiceFromConfig() (*app.Service, error) {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return nil, err
	}

	var repos []driven.IssueRepository
	for name, backend := range cfg.Backends {
		switch name {
		case "linear":
			key := backend.ResolveKey()
			if key == "" {
				return nil, fmt.Errorf("linear: %s is not set", backend.APIKeyEnv)
			}
			team := backend.Team
			if team == "" {
				team = "HEG"
			}
			repo, err := linear.New(key, team)
			if err != nil {
				return nil, fmt.Errorf("linear: %w", err)
			}
			repos = append(repos, repo)
		default:
			return nil, fmt.Errorf("unsupported backend: %s", name)
		}
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("no backends configured in config file")
	}
	return app.NewService(repos...), nil
}

func newServiceFromEnv() (*app.Service, error) {
	key := os.Getenv("LINEAR_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("no backends configured (set LINEAR_API_KEY or create config file)")
	}

	team := os.Getenv("LINEAR_TEAM")
	if team == "" {
		team = "HEG"
	}

	repo, err := linear.New(key, team)
	if err != nil {
		return nil, fmt.Errorf("linear: %w", err)
	}

	return app.NewService(repo), nil
}

func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "emcee",
	Short: "Master of Ceremonies — unified issue management",
	Long:  "Emcee provides a single CLI and MCP server for managing issues across Linear, GitHub, and Jira.",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		filter := domain.ListFilter{
			Project:  flagProject,
			Status:   domain.Status(flagStatus),
			Labels:   flagLabels,
			Limit:    flagLimit,
		}
		issues, err := svc.List(context.Background(), flagBackend, filter)
		if err != nil {
			return err
		}
		return printIssues(issues)
	},
}

var getCmd = &cobra.Command{
	Use:   "get <ref>",
	Short: "Get issue by ref (e.g. linear:HEG-17)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		issue, err := svc.Get(context.Background(), args[0])
		if err != nil {
			return err
		}
		return printIssue(issue)
	},
}

var createCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		input := domain.CreateInput{
			Title:       args[0],
			Description: flagDescription,
			Priority:    domain.ParsePriority(flagPriority),
			Labels:      flagLabels,
			ParentID:    flagParent,
			ProjectID:   flagProjectID,
			Assignee:    flagAssignee,
		}
		issue, err := svc.Create(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created %s: %s\n", issue.Ref, issue.Title)
		if issue.URL != "" {
			fmt.Printf("  %s\n", issue.URL)
		}
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <ref>",
	Short: "Update an issue by ref",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		var input domain.UpdateInput
		if flagStatus != "" {
			s := domain.Status(flagStatus)
			input.Status = &s
		}
		if flagPriority != "" {
			p := domain.ParsePriority(flagPriority)
			input.Priority = &p
		}
		issue, err := svc.Update(context.Background(), args[0], input)
		if err != nil {
			return err
		}
		fmt.Printf("Updated %s: %s [%s]\n", issue.Ref, issue.Title, issue.Status)
		return nil
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search issues",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		issues, err := svc.Search(context.Background(), flagBackend, args[0], flagLimit)
		if err != nil {
			return err
		}
		return printIssues(issues)
	},
}

func printIssues(issues []domain.Issue) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(issues)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REF\tSTATUS\tPRI\tTITLE\tLABELS")
	for _, i := range issues {
		labels := strings.Join(i.Labels, ",")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", i.Ref, i.Status, i.Priority, i.Title, labels)
	}
	return w.Flush()
}

func printIssue(issue *domain.Issue) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(issue)
	}
	fmt.Printf("%s  %s\n", issue.Ref, issue.Title)
	fmt.Printf("  Status:   %s\n", issue.Status)
	fmt.Printf("  Priority: %s\n", issue.Priority)
	if len(issue.Labels) > 0 {
		fmt.Printf("  Labels:   %s\n", strings.Join(issue.Labels, ", "))
	}
	if issue.Assignee != "" {
		fmt.Printf("  Assignee: %s\n", issue.Assignee)
	}
	if issue.URL != "" {
		fmt.Printf("  URL:      %s\n", issue.URL)
	}
	if issue.Description != "" {
		fmt.Printf("\n%s\n", issue.Description)
	}
	return nil
}

// --- Document commands ---

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Manage documents",
}

var docListCmd = &cobra.Command{
	Use:   "list",
	Short: "List documents",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		docs, err := svc.ListDocuments(context.Background(), flagBackend, domain.DocumentListFilter{Limit: flagLimit})
		if err != nil {
			return err
		}
		return printDocuments(docs)
	},
}

var docCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a document",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		content := flagContent
		if flagFile != "" && content == "" {
			data, err := os.ReadFile(flagFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			content = string(data)
		}
		input := domain.DocumentCreateInput{
			Title:     args[0],
			Content:   content,
			ProjectID: flagProjectID,
		}
		doc, err := svc.CreateDocument(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created document: %s\n", doc.Title)
		if doc.URL != "" {
			fmt.Printf("  %s\n", doc.URL)
		}
		return nil
	},
}

func printDocuments(docs []domain.Document) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(docs)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tURL")
	for _, d := range docs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.ID, d.Title, d.URL)
	}
	return w.Flush()
}

// --- Project commands ---

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		projs, err := svc.ListProjects(context.Background(), flagBackend, domain.ProjectListFilter{Limit: flagLimit})
		if err != nil {
			return err
		}
		return printProjects(projs)
	},
}

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		input := domain.ProjectCreateInput{
			Name:        args[0],
			Description: flagDescription,
		}
		proj, err := svc.CreateProject(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created project: %s\n", proj.Name)
		if proj.URL != "" {
			fmt.Printf("  %s\n", proj.URL)
		}
		return nil
	},
}

func printProjects(projs []domain.Project) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(projs)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tURL")
	for _, p := range projs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Name, p.Status, p.URL)
	}
	return w.Flush()
}

// --- Initiative commands ---

var initiativeCmd = &cobra.Command{
	Use:   "initiative",
	Short: "Manage initiatives",
}

var initiativeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List initiatives",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		inits, err := svc.ListInitiatives(context.Background(), flagBackend, domain.InitiativeListFilter{Limit: flagLimit})
		if err != nil {
			return err
		}
		return printInitiatives(inits)
	},
}

var initiativeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an initiative",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		input := domain.InitiativeCreateInput{
			Name:        args[0],
			Description: flagDescription,
		}
		init, err := svc.CreateInitiative(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created initiative: %s\n", init.Name)
		return nil
	},
}

func printInitiatives(inits []domain.Initiative) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(inits)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS")
	for _, i := range inits {
		fmt.Fprintf(w, "%s\t%s\t%s\n", i.ID, i.Name, i.Status)
	}
	return w.Flush()
}

// --- Label commands ---

var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage labels",
}

var labelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List labels",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		labels, err := svc.ListLabels(context.Background(), flagBackend)
		if err != nil {
			return err
		}
		return printLabels(labels)
	},
}

var labelCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		input := domain.LabelCreateInput{
			Name:  args[0],
			Color: flagColor,
		}
		label, err := svc.CreateLabel(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created label: %s\n", label.Name)
		return nil
	},
}

func printLabels(labels []domain.Label) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(labels)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tCOLOR")
	for _, l := range labels {
		fmt.Fprintf(w, "%s\t%s\t%s\n", l.ID, l.Name, l.Color)
	}
	return w.Flush()
}

// --- Bulk create command ---

var bulkCreateCmd = &cobra.Command{
	Use:   "bulk-create",
	Short: "Create issues in bulk from JSON input",
	Long:  "Reads a JSON array of issue objects from stdin or --file and creates them in batches.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		var data []byte
		if flagFile != "" {
			data, err = os.ReadFile(flagFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
		} else {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
		}

		var inputs []domain.CreateInput
		if err := json.Unmarshal(data, &inputs); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		result, err := svc.BulkCreateIssues(context.Background(), flagBackend, inputs)
		if err != nil {
			return err
		}

		fmt.Printf("Created %d/%d issues in %d batches\n", len(result.Created), result.Total, result.Batches)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  error: %s\n", e)
		}
		if flagJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		return nil
	},
}

// --- Bulk update command ---

var bulkUpdateCmd = &cobra.Command{
	Use:   "bulk-update <ref...>",
	Short: "Update multiple issues at once",
	Long:  "Apply the same status/priority change to multiple issues. Example: emcee bulk-update --status done linear:HEG-184 linear:HEG-185",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}

		var input domain.UpdateInput
		if flagStatus != "" {
			s := domain.Status(flagStatus)
			input.Status = &s
		}
		if flagPriority != "" {
			p := domain.ParsePriority(flagPriority)
			input.Priority = &p
		}

		var errors []string
		var updated int
		for _, ref := range args {
			_, err := svc.Update(context.Background(), ref, input)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ref, err))
				continue
			}
			updated++
		}

		fmt.Printf("Updated %d/%d issues\n", updated, len(args))
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  error: %s\n", e)
		}
		if len(errors) > 0 {
			return fmt.Errorf("%d issues failed", len(errors))
		}
		return nil
	},
}

// --- Children command ---

var childrenCmd = &cobra.Command{
	Use:   "children <ref>",
	Short: "List sub-issues of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		issues, err := svc.ListChildren(context.Background(), args[0])
		if err != nil {
			return err
		}
		return printIssues(issues)
	},
}

// --- Import command ---

var importCmd = &cobra.Command{
	Use:   "import <file.md>",
	Short: "Create an issue from a markdown file",
	Long:  "Reads a markdown file, uses the first heading as the title and the rest as the description.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		title, desc := parseMarkdown(string(data))
		input := domain.CreateInput{
			Title:       title,
			Description: desc,
			Priority:    domain.ParsePriority(flagPriority),
			Labels:      flagLabels,
			ParentID:    flagParent,
			ProjectID:   flagProjectID,
			Assignee:    flagAssignee,
		}
		issue, err := svc.Create(context.Background(), flagBackend, input)
		if err != nil {
			return err
		}
		fmt.Printf("Created %s: %s\n", issue.Ref, issue.Title)
		if issue.URL != "" {
			fmt.Printf("  %s\n", issue.URL)
		}
		return nil
	},
}

func parseMarkdown(content string) (title, body string) {
	lines := strings.SplitN(content, "\n", 2)
	title = strings.TrimSpace(lines[0])
	title = strings.TrimLeft(title, "# ")
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}
	return
}

// --- Serve command ---

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server over stdio",
	Long:  "Starts emcee as an MCP (Model Context Protocol) server over stdio, exposing issue management tools to AI agents.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newService()
		if err != nil {
			return err
		}
		return mcpserver.Serve(svc)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagBackend, "backend", "b", "linear", "Backend to use")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "Config file path (default ~/.config/emcee/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON")

	listCmd.Flags().StringVarP(&flagProject, "project", "p", "", "Project filter")
	listCmd.Flags().StringVarP(&flagStatus, "status", "s", "", "Status filter")
	listCmd.Flags().StringSliceVarP(&flagLabels, "label", "l", nil, "Label filter")
	listCmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "Max results")

	createCmd.Flags().StringVar(&flagPriority, "priority", "", "Priority (urgent/high/medium/low)")
	createCmd.Flags().StringSliceVarP(&flagLabels, "label", "l", nil, "Labels")
	createCmd.Flags().StringVar(&flagParent, "parent", "", "Parent issue ID for sub-issues")
	createCmd.Flags().StringVar(&flagAssignee, "assignee", "", "Assignee name")
	createCmd.Flags().StringVar(&flagDescription, "description", "", "Issue description")
	createCmd.Flags().StringVar(&flagProjectID, "project-id", "", "Project ID")

	updateCmd.Flags().StringVarP(&flagStatus, "status", "s", "", "New status")
	updateCmd.Flags().StringVar(&flagPriority, "priority", "", "New priority")

	searchCmd.Flags().IntVarP(&flagLimit, "limit", "n", 20, "Max results")

	// Document subcommands
	docListCmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "Max results")
	docCreateCmd.Flags().StringVar(&flagContent, "content", "", "Document content")
	docCreateCmd.Flags().StringVarP(&flagFile, "file", "f", "", "Read content from file")
	docCreateCmd.Flags().StringVar(&flagProjectID, "project-id", "", "Project ID to link to")
	docCmd.AddCommand(docListCmd, docCreateCmd)

	// Project subcommands
	projectListCmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "Max results")
	projectCreateCmd.Flags().StringVar(&flagDescription, "description", "", "Project description")
	projectCmd.AddCommand(projectListCmd, projectCreateCmd)

	// Initiative subcommands
	initiativeListCmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "Max results")
	initiativeCreateCmd.Flags().StringVar(&flagDescription, "description", "", "Initiative description")
	initiativeCmd.AddCommand(initiativeListCmd, initiativeCreateCmd)

	// Label subcommands
	labelCreateCmd.Flags().StringVar(&flagColor, "color", "", "Label color (hex)")
	labelCmd.AddCommand(labelListCmd, labelCreateCmd)

	// Bulk create
	bulkCreateCmd.Flags().StringVarP(&flagFile, "file", "f", "", "Read JSON from file instead of stdin")

	// Import
	importCmd.Flags().StringVar(&flagPriority, "priority", "", "Priority")
	importCmd.Flags().StringSliceVarP(&flagLabels, "label", "l", nil, "Labels")
	importCmd.Flags().StringVar(&flagParent, "parent", "", "Parent issue ID")
	importCmd.Flags().StringVar(&flagProjectID, "project-id", "", "Project ID")
	importCmd.Flags().StringVar(&flagAssignee, "assignee", "", "Assignee name")

	rootCmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, searchCmd, serveCmd)
	rootCmd.AddCommand(docCmd, projectCmd, initiativeCmd, labelCmd)
	// Bulk update
	bulkUpdateCmd.Flags().StringVarP(&flagStatus, "status", "s", "", "New status for all issues")
	bulkUpdateCmd.Flags().StringVar(&flagPriority, "priority", "", "New priority for all issues")

	rootCmd.AddCommand(bulkCreateCmd, bulkUpdateCmd, childrenCmd, importCmd)
}
