// Package cli implements the driver (inbound) adapter for the cobra CLI.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/DanyPops/emcee/internal/adapter/driven/linear"
	mcpserver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
	"github.com/DanyPops/emcee/internal/app"
	"github.com/DanyPops/emcee/internal/config"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driver"
	"github.com/spf13/cobra"
)

var (
	flagBackend  string
	flagConfig   string
	flagProject  string
	flagStatus   string
	flagPriority string
	flagLabels   []string
	flagLimit    int
	flagJSON     bool
)

func newService() (driver.IssueService, error) {
	if config.Exists(flagConfig) {
		return newServiceFromConfig()
	}
	return newServiceFromEnv()
}

func newServiceFromConfig() (driver.IssueService, error) {
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

func newServiceFromEnv() (driver.IssueService, error) {
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
			Title:    args[0],
			Priority: domain.ParsePriority(flagPriority),
			Labels:   flagLabels,
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

	updateCmd.Flags().StringVarP(&flagStatus, "status", "s", "", "New status")
	updateCmd.Flags().StringVar(&flagPriority, "priority", "", "New priority")

	searchCmd.Flags().IntVarP(&flagLimit, "limit", "n", 20, "Max results")

	rootCmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, searchCmd, serveCmd)
}

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
