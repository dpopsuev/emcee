# emcee

All Ceremonies in one place — a unified CLI + MCP server for issue/project management across platforms.

One interface to rule them all: **Linear**, **GitHub Issues**, **GitLab Issues**, **Jira**.

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

### Environment Variables

```bash
# Linear
export LINEAR_API_KEY=lin_api_xxx
export LINEAR_TEAM=HEG

# GitHub
export GITHUB_TOKEN=ghp_xxx
export GITHUB_OWNER=DanyPops
export GITHUB_REPO=emcee

# GitLab (SaaS)
export GITLAB_TOKEN=glpat-xxx
export GITLAB_PROJECT=12345678  # or namespace/project

# GitLab (Self-hosted)
export GITLAB_TOKEN=glpat-xxx
export GITLAB_PROJECT=namespace/project
export GITLAB_URL=https://gitlab.company.com

# Jira
export JIRA_API_TOKEN=xxx
export JIRA_URL=https://mycompany.atlassian.net
export JIRA_EMAIL=me@company.com
export JIRA_PROJECT=PROJ
```

### Config File

```yaml
# ~/.config/emcee/config.yaml
backends:
  linear:
    api_key_env: LINEAR_API_KEY
    team: HEG
  
  github:
    token_env: GITHUB_TOKEN
    owner: DanyPops
    team: emcee  # GitHub uses 'team' field for repo name
  
  gitlab:
    token_env: GITLAB_TOKEN
    team: namespace/project  # GitLab uses 'team' field for project ID
    url: https://gitlab.com  # optional, defaults to gitlab.com
  
  jira:
    url: https://mycompany.atlassian.net
    token_env: JIRA_API_TOKEN
    email: me@company.com
    team: PROJ

projects:
  hegemony:
    backend: linear
    project: HEG
  
  emcee:
    backend: github
    repo: DanyPops/emcee
  
  work:
    backend: jira
    project: PROJ
```

### GitHub Setup

1. Create a Personal Access Token at https://github.com/settings/tokens
2. Grant `repo` scope (full control of private repositories)
3. Export as `GITHUB_TOKEN` or add to config file
4. Set `GITHUB_OWNER` (your username or org name) and `GITHUB_REPO` (repository name)

### GitLab Setup

1. **GitLab SaaS (gitlab.com)**:
   - Create a Personal Access Token at https://gitlab.com/-/profile/personal_access_tokens
   - Grant `api` scope (full API access)
   - Export as `GITLAB_TOKEN` or add to config file
   - Set `GITLAB_PROJECT` (numeric ID or `namespace/project` format)

2. **Self-Hosted GitLab**:
   - Same as above, but also set `GITLAB_URL` (e.g., `https://gitlab.company.com`)
   - Or add `url:` field in config file

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
  Linear  GitHub   GitLab     Jira
  adapter adapter  adapter   adapter
```
