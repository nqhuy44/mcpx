# External Skills

These skills are workflows built on top of **third-party or community MCP servers** — not the servers in `servers/`. They don't require you to build anything; they assume those external servers are already running and registered with the proxy.

The skill definitions live in `skills/external/` as `.claude/commands/`-compatible prompt files or `skill.yaml` orchestration definitions.

---

## Concept: External vs. Internal Skills

| | Internal Skills | External Skills |
|---|---|---|
| **Tools used** | Servers built in `servers/` | Third-party MCP servers |
| **Token optimization** | Full control — we own the server | Depends on the external server's design |
| **Portability** | mcpx-only | Works with any MCP host (Cursor, Claude Desktop, etc.) |
| **Location** | `skills/` | `skills/external/` |

When an external MCP server returns bloated data, wrap the call inside an `exec_run` or `llm_summarize` step to filter it before it hits the cloud context.

---

## Directory Layout

```
skills/external/
├── README.md
├── filesystem/
│   ├── skill.yaml
│   └── prompts/
├── browser/
│   ├── skill.yaml
│   └── prompts/
├── postgres/
│   ├── skill.yaml
│   └── prompts/
├── slack/
│   ├── skill.yaml
│   └── prompts/
├── linear/
│   ├── skill.yaml
│   └── prompts/
├── sentry/
│   ├── skill.yaml
│   └── prompts/
└── web-search/
    ├── skill.yaml
    └── prompts/
```

---

## Filesystem Skills (`mcp-server-filesystem`)

**External server**: `@modelcontextprotocol/server-filesystem`  
**What it exposes**: read/write/list files on the local filesystem

### Skill: `audit-config-files`
Scan all config files in a project for hardcoded secrets, deprecated keys, or inconsistent environment variable names.

```yaml
name: audit-config-files
description: Scan config files for secrets, deprecated keys, mismatches

steps:
  - id: find_configs
    tool: filesystem_list
    args:
      path: "{{ input.project_root }}"
      pattern: "**/{*.env,*.yaml,*.toml,*.json,docker-compose*}"

  - id: read_configs
    tool: filesystem_read_multiple
    args:
      paths: "{{ steps.find_configs.output }}"

  - id: audit
    model: cloud
    prompt: prompts/audit-configs.md
    input:
      files: "{{ steps.read_configs.output }}"
```

**Token guard**: `filesystem_read_multiple` can return enormous payloads. If total file size exceeds 20KB, route through `exec_run` first to strip comments and blank lines.

### Skill: `explain-project-structure`
Generate a human-readable map of a project for onboarding. Pairs well with `onboard-codebase` from internal skills.

---

## Browser / Web Skills (`mcp-server-puppeteer`, `mcp-server-playwright`)

**External server**: `@modelcontextprotocol/server-puppeteer` or `mcp-playwright`  
**What it exposes**: navigate pages, click, fill forms, screenshot, scrape

### Skill: `check-deployed-app`
After a deploy, navigate to the app URL, run a smoke-test checklist, and report pass/fail with screenshots.

```yaml
name: check-deployed-app
description: Post-deploy smoke test via headless browser

steps:
  - id: open_app
    tool: browser_navigate
    args:
      url: "{{ input.app_url }}"

  - id: check_login
    tool: browser_click
    args:
      selector: "[data-testid=login-button]"

  - id: screenshot
    tool: browser_screenshot

  - id: report
    model: local
    prompt: prompts/smoke-test-report.md
    input:
      page_content: "{{ steps.open_app.output }}"
      screenshot: "{{ steps.screenshot.output }}"
```

**Token guard**: Never pass raw `page_content` (full DOM HTML) to cloud model. Local model or `exec_run` should extract only the relevant assertions before the cloud sees anything.

### Skill: `scrape-and-summarize`
Fetch a documentation page or changelog URL and return a condensed summary.

---

## Database Skills (`mcp-server-postgres`, `mcp-server-sqlite`)

**External server**: `mcp-server-postgres` or `@modelcontextprotocol/server-sqlite`  
**What it exposes**: schema inspection, read-only queries

### Skill: `explain-schema`
Given a database, produce a LEAN-formatted summary of tables, columns, indexes, and foreign key relationships — without exposing raw `information_schema` dumps.

```yaml
name: explain-schema
description: Summarize database schema in LEAN format

steps:
  - id: get_tables
    tool: db_query
    args:
      sql: "SELECT table_name, column_name, data_type FROM information_schema.columns ORDER BY table_name, ordinal_position"

  - id: get_fk
    tool: db_query
    args:
      sql: "SELECT ... FROM information_schema.referential_constraints ..."

  - id: format
    model: local
    prompt: prompts/format-schema.md
    input:
      tables: "{{ steps.get_tables.output }}"
      foreign_keys: "{{ steps.get_fk.output }}"
```

**Output** (LEAN):
```
@table users
cols: id(uuid,pk), email(text,unique), created_at(timestamp)
refs: none

@table orders
cols: id(uuid,pk), user_id(uuid), total(numeric), status(text)
refs: user_id → users.id
```

### Skill: `slow-query-triage`
Identify the slowest queries from `pg_stat_statements`, explain the top offenders, and suggest indexes.

---

## Slack Skills (`mcp-server-slack`)

**External server**: `mcp-server-slack`  
**What it exposes**: post messages, read channels, list users

### Skill: `post-deploy-announcement`
After a successful deploy, compose and post a structured release announcement to a Slack channel.

```yaml
name: post-deploy-announcement
steps:
  - id: get_changes
    tool: git_list
    args:
      resource: commits
      since: "{{ input.last_tag }}"

  - id: draft
    model: local
    prompt: prompts/draft-slack-announcement.md
    input:
      commits: "{{ steps.get_changes.output }}"
      version: "{{ input.version }}"
      env: "{{ input.environment }}"

  - id: post
    tool: slack_post_message
    args:
      channel: "{{ input.channel }}"
      text: "{{ steps.draft.output }}"
```

### Skill: `triage-alert-thread`
Read an active incident Slack thread, extract the timeline and key decisions, post a structured summary as a reply.

---

## Linear / Jira Skills (`mcp-linear`, `mcp-atlassian`)

**External server**: `mcp-linear` or `mcp-server-atlassian`  
**What it exposes**: create/update issues, list projects, search

### Skill: `create-bug-from-sentry`
Takes a Sentry error URL, fetches the stack trace, and creates a Linear/Jira bug ticket with structured reproduction steps.

```yaml
name: create-bug-from-sentry
steps:
  - id: fetch_error
    tool: sentry_get
    args:
      resource: issue
      url: "{{ input.sentry_url }}"

  - id: find_code
    tool: code_search
    args:
      query: "{{ steps.fetch_error.culprit }}"

  - id: draft_ticket
    model: cloud
    prompt: prompts/draft-bug-ticket.md
    input:
      error: "{{ steps.fetch_error.output }}"
      code_context: "{{ steps.find_code.output }}"

  - id: create
    tool: linear_create
    args:
      resource: issue
      title: "{{ steps.draft_ticket.title }}"
      description: "{{ steps.draft_ticket.body }}"
      team: "{{ input.team }}"
      priority: "{{ steps.draft_ticket.priority }}"
```

### Skill: `link-pr-to-ticket`
Detect the Linear/Jira ticket ID in a PR branch name or description, and post the PR link back to the ticket.

---

## Sentry Skills (`mcp-sentry`)

**External server**: `mcp-sentry`  
**What it exposes**: fetch issues, events, stack traces

### Skill: `sentry-weekly-report`
Aggregate the top 10 errors by volume from the past 7 days across all projects, group by error type, and produce a LEAN report.

**Token guard**: Sentry issue lists can be enormous. Server-side filter to: error title + count + first_seen + last_seen + culprit only. Strip user context, breadcrumbs, raw request data.

---

## Web Search Skills (`mcp-server-brave-search`, `mcp-tavily`)

**External server**: Brave Search MCP or Tavily MCP  
**What it exposes**: web search results

### Skill: `research-library`
Research a library or tool, compare alternatives, and return a structured recommendation. Uses local model to process search results before cloud synthesizes the recommendation.

```yaml
name: research-library
steps:
  - id: search_main
    tool: search_web
    args:
      query: "{{ input.topic }} best practices 2025"
      count: 5

  - id: search_alternatives
    tool: search_web
    args:
      query: "{{ input.topic }} alternatives comparison"
      count: 5

  - id: compress_results
    model: local
    prompt: prompts/compress-search-results.md
    input:
      results: "{{ steps.search_main.output + steps.search_alternatives.output }}"

  - id: recommend
    model: cloud
    prompt: prompts/write-recommendation.md
    input:
      compressed: "{{ steps.compress_results.output }}"
      context: "{{ input.use_case }}"
```

**Token guard**: Raw search result snippets from Brave/Tavily are redundant and verbose. Always route through local model compression before the cloud sees them. Target: 10 search results → ~400 tokens after compression.

---

## Token Guard Pattern for External Skills

Since we don't control external MCP servers, always apply this defensive pattern when their output feeds into a cloud model:

```yaml
# After any external tool call that returns large data:
- id: compress
  model: local          # free, runs on-device
  prompt: |
    Extract only the essential facts from this data.
    Remove all metadata, timestamps, IDs, and redundant fields.
    Output as LEAN blocks.
  input:
    raw: "{{ steps.external_call.output }}"

# Then pass compressed output to cloud:
- id: analyze
  model: cloud
  input:
    data: "{{ steps.compress.output }}"   # never steps.external_call.output directly
```

This ensures a misconfigured or verbose external MCP server can't blow up the cloud context window.
