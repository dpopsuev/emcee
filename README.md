# emcee

All Ceremonies in one place — a unified CLI + MCP server for issue management and defect triage across platforms.

**Linear** | **GitHub** | **GitLab** | **Jira**

CI/CD operations (Jenkins, GitHub Actions, GitLab CI, Report Portal) moved to [Conty](https://github.com/dpopsuev/conty).

## Install

```bash
go install github.com/DanyPops/emcee@latest
```

## What it does

- **Issues** — list, get, create, update, search, children, bulk ops across all backends
- **Comments** — read and add comments inline with issue retrieval
- **Issue Links** — expose Jira issue links (blocks, relates, clones) and external links (PRs, commits); create links programmatically
- **Staging** — stage issues locally before submitting, auto-stage on failure, push when ready
- **Pull Requests** — list PRs/MRs across GitHub and GitLab with date/author/repo filters
- **Triage** — recursive cross-backend defect lifecycle graph (rate-limited, allowlist-filtered)
- **Ledger** — persistent SQLite artifact index with FTS5 full-text search, similarity, passive + active ingest
- **Multi-instance** — run multiple Jira/etc instances via named profiles in config

## CLI

```bash
emcee list -b jira --status open --limit 20
emcee get jira:PROJ-42
emcee create -b linear "Fix the thing" --priority high --label bug
emcee update jira:PROJ-42 --status done --resolution "Done"
emcee search -b github "auth bug" --limit 10

# Staging
emcee create -b jira "Draft issue" --stage
emcee stage list
emcee push stg_abc123

# Comments
emcee comment list jira:PROJ-42
emcee comment add jira:PROJ-42 "Looks good, shipping"

# Bulk
emcee bulk-create -b jira -f issues.json
emcee bulk-update --status done jira:PROJ-1 jira:PROJ-2

# JQL (Jira)
emcee jql -b jira "project = PROJ AND status = Open" --limit 50

# Fields
emcee fields -b jira --json
```

## MCP Server

```bash
emcee serve   # stdio MCP server for AI agents
```

Three tools exposed: `emcee` (25+ actions), `emcee_manage` (entities + backend management), `emcee_health`.

### Key MCP actions

| Category | Actions |
|----------|---------|
| Issues | `list`, `get`, `create`, `update`, `search`, `children` |
| Bulk | `bulk_create`, `bulk_update` |
| Comments | `comments`, `comment_add` |
| Staging | `stage`, `stage_list`, `stage_show`, `stage_patch`, `stage_drop`, `push`, `push_all` |
| Issue Links | `link_issue` |
| PRs | `prs` |
| Triage | `triage`, `triage_config`, `triage_config_set` |
| Ledger | `ledger_list`, `ledger_get`, `ledger_search`, `ledger_similar`, `ledger_ingest`, `ledger_stats` |
| Discovery | `fields`, `jql` |

## Configuration

### Environment Variables

```bash
# Linear
export LINEAR_API_KEY=lin_api_xxx
export LINEAR_TEAM=HEG

# GitHub (token optional for public repo reads)
export GITHUB_TOKEN=ghp_xxx
export GITHUB_OWNER=DanyPops

# GitLab (token optional for public project reads)
export GITLAB_TOKEN=glpat-xxx
export GITLAB_PROJECT=namespace/project
export GITLAB_URL=https://gitlab.company.com  # optional, defaults to gitlab.com

# Jira
export JIRA_API_TOKEN=xxx
export JIRA_URL=https://mycompany.atlassian.net
export JIRA_EMAIL=me@company.com
export JIRA_PROJECT=PROJ
```

### Config File (multi-instance)

```yaml
# ~/.config/emcee/config.yaml
backends:
  jira:
    url: https://mycompany.atlassian.net
    token_env: JIRA_API_TOKEN
    email: me@company.com
    team: PROJ

  github:
    token_env: GITHUB_TOKEN
    owner: DanyPops

  gitlab:
    token_env: GITLAB_TOKEN
    team: namespace/project
    url: https://gitlab.company.com
```

Backend names become the `--backend` / `backend` param. The `type` field maps to the adapter when the name differs.

## Persistence

The **Ledger** stores every artifact Emcee sees at `$XDG_DATA_HOME/emcee/ledger.db` (default: `~/.local/share/emcee/ledger.db`). SQLite WAL mode, FTS5 full-text search. Passive ingest on every get/list/search, plus active `ledger_ingest` for manual deposits.

## Architecture

```
Driver (inbound)          Core               Driven (outbound)
┌────────────┐      ┌─────────────┐      ┌──────────────────┐
│ CLI (cobra) │─────>│             │─────>│ Linear           │
│ MCP (stdio) │─────>│ App Service │─────>│ GitHub           │
└────────────┘      │             │─────>│ GitLab           │
                    │  Triage     │─────>│ Jira             │
                    │  Staging    │─────>│ SQLite (Ledger)  │
                    │  Ledger     │─────>│ Cache (LRU)      │
                    └─────────────┘      └──────────────────┘
```

Hexagonal architecture. Ports define contracts, adapters implement them. Multi-instance backends via config with runtime hot-reload.
