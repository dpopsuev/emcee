# emcee

All Ceremonies in one place — a unified CLI + MCP server for issue, build, and test management across platforms.

**Linear** | **GitHub** | **GitLab** | **Jira** | **Report Portal** | **Jenkins**

## Install

```bash
go install github.com/DanyPops/emcee@latest
```

## What it does

- **Issues** — list, get, create, update, search, children, bulk ops across all backends
- **Comments** — read and add comments inline with issue retrieval
- **Staging** — stage issues locally before submitting, auto-stage on failure, push when ready
- **Jenkins CI** — jobs, builds, build history, logs, test results, queue, folders, pipelines, artifacts, nodes, views
- **Report Portal** — launches, test items, defect updates
- **Pull Requests** — list PRs/MRs across GitHub and GitLab with date/author/repo filters
- **Triage** — recursive cross-backend defect lifecycle graph (rate-limited, allowlist-filtered)
- **Ledger** — persistent SQLite artifact index with FTS5 full-text search, passive + active ingest
- **Multi-instance** — run multiple Jenkins/Jira/etc instances via named profiles in config

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

Three tools exposed: `emcee` (55+ actions), `emcee_manage` (entities + backend management), `emcee_health`.

### Key MCP actions

| Category | Actions |
|----------|---------|
| Issues | `list`, `get`, `create`, `update`, `search`, `children` |
| Bulk | `bulk_create`, `bulk_update` |
| Comments | `comments`, `comment_add` |
| Staging | `stage`, `stage_list`, `stage_show`, `stage_patch`, `stage_drop`, `push`, `push_all` |
| Jenkins | `jobs`, `job_get`, `build_trigger`, `build_get`, `build_log`, `builds`, `build_last`, `build_stop`, `job_params`, `folder_jobs`, `nodes`, `views`, ... |
| Pipelines | `pipeline_runs`, `pipeline_run_get`, `pipeline_inputs`, `pipeline_input_approve` |
| Report Portal | `launches`, `launch_get`, `test_items`, `defect_update` |
| PRs | `prs` |
| Triage | `triage`, `triage_config`, `triage_config_set` |
| Ledger | `ledger_list`, `ledger_get`, `ledger_search`, `ledger_ingest`, `ledger_stats` |
| Discovery | `fields`, `jql` |

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

# GitLab
export GITLAB_TOKEN=glpat-xxx
export GITLAB_PROJECT=namespace/project
export GITLAB_URL=https://gitlab.company.com  # optional, defaults to gitlab.com

# Jira
export JIRA_API_TOKEN=xxx
export JIRA_URL=https://mycompany.atlassian.net
export JIRA_EMAIL=me@company.com
export JIRA_PROJECT=PROJ

# Report Portal
export RP_TOKEN=xxx
export RP_URL=https://reportportal.company.com
export RP_PROJECT=my-project

# Jenkins
export JENKINS_API_KEY=xxx
export JENKINS_URL=https://jenkins.company.com
export JENKINS_USER=me
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

  jenkins-ci:
    type: jenkins
    url: https://jenkins-ci.company.com
    api_key: your-api-token
    email: me

  jenkins-auto:
    type: jenkins
    url: https://jenkins-auto.company.com
    api_key: your-other-token
    email: me

  reportportal:
    type: reportportal
    url: https://reportportal.company.com
    token_env: RP_TOKEN
    team: my-project

  github:
    token_env: GITHUB_TOKEN
    owner: DanyPops
    team: emcee

  gitlab:
    token_env: GITLAB_TOKEN
    team: namespace/project
    url: https://gitlab.company.com
```

Backend names become the `--backend` / `backend` param. The `type` field maps to the adapter when the name differs (e.g. `jenkins-ci` with `type: jenkins`).

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
                    │  Staging    │─────>│ Report Portal    │
                    │  Ledger     │─────>│ Jenkins          │
                    └─────────────┘      │ SQLite (Ledger)  │
                                         │ Cache (LRU)      │
                                         └──────────────────┘
```

Hexagonal architecture. Ports define contracts, adapters implement them. Multi-instance backends via config with runtime hot-reload.
