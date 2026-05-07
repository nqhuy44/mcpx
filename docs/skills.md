# Skills

Skills are higher-level, composable workflows that orchestrate multiple MCP server tools + prompt logic to accomplish a complete engineering task. They sit above the tool layer.

## Skills vs. MCP Servers

| | MCP Servers | Skills |
|---|---|---|
| **What** | Atomic tools (get, list, analyze) | Multi-step workflows |
| **Who calls them** | The LLM, one tool at a time | The agent, as a single intent |
| **State** | Stateless | Can hold intermediate state |
| **Examples** | `git_get pr`, `cicd_analyze` | "Review this PR", "Debug the pipeline" |
| **Implementation** | Go/Rust/Python binary | YAML workflow + prompt templates |

An analogy: MCP servers are functions; skills are programs built from those functions.

## Directory Layout

```
skills/
├── README.md
├── _templates/                  # Reusable prompt fragments
│   ├── system-base.md
│   ├── code-reviewer.md
│   └── devops-analyst.md
│
├── review-pr/
│   ├── skill.yaml               # Skill definition
│   └── prompts/
│       ├── summarize-diff.md
│       └── generate-review.md
│
├── debug-pipeline/
│   ├── skill.yaml
│   └── prompts/
│       └── triage-failure.md
│
├── refactor-module/
│   ├── skill.yaml
│   └── prompts/
│
├── release-notes/
│   ├── skill.yaml
│   └── prompts/
│
├── onboard-codebase/
│   ├── skill.yaml
│   └── prompts/
│
└── security-audit/
    ├── skill.yaml
    └── prompts/
```

## Skill Definition Format (`skill.yaml`)

```yaml
name: review-pr
version: 1.0.0
description: Full code review for a pull request
trigger: slash  # slash | auto | webhook

input:
  - name: pr_number
    type: integer
    required: true
  - name: repo
    type: string
    default: current

routing:
  # Which model handles each step
  local_model: qwen3:14b      # used for summarization steps
  cloud_model: claude          # used for final review generation

steps:
  - id: fetch_pr
    tool: git_get
    args:
      resource: pr
      number: "{{ input.pr_number }}"
      repo: "{{ input.repo }}"

  - id: fetch_diff
    tool: git_get
    args:
      resource: pr_diff
      number: "{{ input.pr_number }}"

  - id: find_changed_symbols
    tool: code_list
    args:
      resource: changed_symbols
      files: "{{ steps.fetch_diff.files_changed }}"

  - id: check_ci
    tool: cicd_get
    args:
      resource: pr_status
      pr: "{{ input.pr_number }}"

  - id: summarize_diff
    model: local          # run on local model, save cloud tokens
    prompt: prompts/summarize-diff.md
    input:
      diff: "{{ steps.fetch_diff.output }}"
      symbols: "{{ steps.find_changed_symbols.output }}"

  - id: generate_review
    model: cloud
    prompt: prompts/generate-review.md
    input:
      pr: "{{ steps.fetch_pr.output }}"
      summary: "{{ steps.summarize_diff.output }}"
      ci_status: "{{ steps.check_ci.output }}"

output:
  format: lean
  value: "{{ steps.generate_review.output }}"
```

## Skill Implementation Options

Skills can be surfaced in three ways depending on the use case:

### 1. Claude Code Slash Commands (simplest)

For skills that are invoked interactively during development. Stored in `.claude/commands/`:

```
.claude/commands/
├── review-pr.md        → /review-pr
├── debug-pipeline.md   → /debug-pipeline
└── release-notes.md    → /release-notes
```

Each `.md` file contains the full prompt + instructions. The agent reads it and executes the tool calls described. No code needed — just well-structured prompts that reference the MCP tools.

Use this for: personal/team developer workflows.

### 2. Orchestrator MCP Server (`servers/orchestrator`)

A dedicated MCP server that exposes skills as tools. The proxy routes `skill_*` tool calls to this server, which then calls other domain servers internally and sequences the steps.

```
cloud LLM
  └──calls──► skill_review_pr
                  │
                  ├──► git-server (fetch PR, diff)
                  ├──► codebase-server (changed symbols)
                  ├──► cicd-server (CI status)
                  └──► llm-server (local summarization)
                          │
                          └── returns composed review
```

Use this for: headless agent pipelines, webhook-triggered automation.

### 3. Webhook / Scheduled Trigger

Skills can be triggered by external events (PR opened, pipeline failed, deploy finished) without any LLM interaction. The orchestrator server receives the webhook, executes the skill, and posts results back (GitHub comment, Slack, Jira ticket).

```yaml
trigger: webhook
on:
  event: pr.opened
  source: github
action: post_review_comment
```

## Planned Skills

### `review-pr`
Full pull request review: fetches diff, identifies changed symbols and their callers, checks CI status, runs summarization locally, generates detailed review with cloud model.

**Steps**: `git_get` → `code_list` → `cicd_get` → local summarize → cloud review  
**Token strategy**: local model handles diff summarization (heavy); cloud handles final synthesis (light)

---

### `debug-pipeline`
CI/CD failure triage: analyzes the failed job log, finds the failing test/line, searches the codebase for context, suggests a fix.

**Steps**: `cicd_analyze` → `code_search` → `code_get` → cloud suggest fix  
**Token strategy**: `cicd_analyze` strips 50K log to ~300 tokens before any LLM sees it

---

### `release-notes`
Generates release notes from all PRs merged since the last tag. Clusters changes by type (feature, fix, infra), links issues.

**Steps**: `git_list commits` → `git_list prs` → local cluster → cloud draft  
**Token strategy**: local model groups/clusters; cloud only writes prose

---

### `onboard-codebase`
Gives a new developer (or a new agent session) a structured orientation to an unfamiliar repository: entry points, key modules, dependency graph, recent change hotspots.

**Steps**: `code_graph root` → `git_list commits` → cloud synthesize overview  
**Output**: LEAN-formatted codebase map

---

### `refactor-module`
Safe, step-by-step refactoring: identifies all usages of a symbol, checks for tests, generates the refactored version, shows a diff preview.

**Steps**: `code_get` → `code_list usages` → `exec_run tests` → cloud refactor → `git_create branch`

---

### `security-audit`
Reviews recent changes for common vulnerabilities: injection, auth gaps, secrets exposure, dependency issues.

**Steps**: `git_get diff` → `code_graph affected` → cloud audit  
**Note**: always routes to cloud model (security reasoning is high-complexity)

---

### `standup-summary`
Generates a daily standup summary: what was committed, what PRs are open, what CI failures are blocking.

**Steps**: `git_list commits` → `cicd_list failed` → `git_list prs open` → local summarize  
**Token strategy**: entirely local model — no cloud needed for this task

## Prompt Template Conventions

All skill prompts follow these rules:

1. **System prompt** references `_templates/system-base.md` for shared persona/constraints
2. **Input data** is always passed as LEAN or TOON — never raw JSON
3. **Output format** is explicitly specified at the end of every prompt
4. **Model routing instructions** are in `skill.yaml`, not in the prompt itself (prompts are model-agnostic)

```markdown
<!-- prompts/summarize-diff.md -->
You are analyzing a code diff for a pull request review.

## Changed Code
{{ input.summary_lean }}

## Task
Identify:
- The primary intent of this change (1 sentence)
- Any potential side effects on callers
- Missing test coverage for edge cases

Output format: LEAN blocks with keys: intent, side_effects[], missing_tests[]
```
