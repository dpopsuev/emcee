# emcee

Master of Ceremonies — a unified CLI + MCP server for issue/project management across platforms.

One interface to rule them all: **Linear**, **GitHub Issues**, **Jira**.

## Install

```bash
go install github.com/DanyPops/emcee@latest
```

## CLI Usage

```bash
# List issues
emcee list --backend linear --project HEG
emcee list --backend github --repo DanyPops/hegemony

# Create issues
emcee create --backend linear --project HEG "Fix the thing" --label bug --priority high
emcee create --backend github --repo DanyPops/hegemony "Fix the thing" --label bug

# Get issue details
emcee get linear:HEG-17
emcee get github:DanyPops/hegemony#42

# Update issues
emcee update linear:HEG-17 --status done
emcee update github:DanyPops/hegemony#42 --label "in-progress"
```

## MCP Server

```bash
emcee serve   # stdio MCP server for AI agents
```

Tools exposed: `emcee_list`, `emcee_create`, `emcee_get`, `emcee_update`, `emcee_search`

## Configuration

```yaml
# ~/.config/emcee/config.yaml
backends:
  linear:
    api_key_env: LINEAR_API_KEY    # reads from env var
  github:
    token_env: GITHUB_TOKEN
  jira:
    url: https://mycompany.atlassian.net
    token_env: JIRA_API_TOKEN
    email: me@company.com

projects:
  hegemony:
    backend: linear
    project: HEG
  emcee:
    backend: github
    repo: DanyPops/emcee
```

## Architecture

```
┌──────────────────────────────────┐
│         CLI (cobra)              │
│         MCP Server (stdio)       │
└──────────┬───────────────────────┘
           │
     ┌─────▼─────┐
     │   Core     │  Unified Issue model
     │   Engine   │  Query/Create/Update
     └─────┬─────┘
           │ Adapter interface
     ┌─────┼──────────┬──────────┐
     ▼     ▼          ▼          ▼
  Linear  GitHub    Jira      (future)
  adapter adapter  adapter    adapter
```
