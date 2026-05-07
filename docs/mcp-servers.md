# MCP Servers & Skills

Each server is an independent binary following the registry-based dispatch pattern. The proxy gateway routes to them — they never talk directly to the LLM host.

**Rule**: Every server exposes ≤12 tool names, regardless of how many underlying operations it supports.

---

## 1. Git Server (`servers/git`)

**Category**: Git / PR Management  
**Language**: Go  
**Status**: Implemented  
**Purpose**: Local git CLI operations and GitHub PR lifecycle — pre-filtering all output to return only actionable fields.

### Exposed Tools

| Tool | Description |
|---|---|
| `git_status` | Working tree status — staged, unstaged, untracked files |
| `git_log` | Commit history; filterable by author, path, date. Returns hash (8 chars), date, author, subject |
| `git_diff` | Diff working tree, staged changes, or a commit range. Supports `--stat` summary mode |
| `git_branch` | List, create, or delete local branches via `action` param |
| `git_show` | Details of a single commit: message, author, date, changed-file summary |
| `git_blame` | Annotate file lines with author + commit. Supports line range |
| `github_pr_list` | List PRs — number, title, author, state, branch, labels, date |
| `github_pr_get` | Full PR details: body (truncated 500 chars), diff stats, mergeable status |
| `github_pr_comment` | Post a comment on a PR or issue |

### Token Optimization

- `git_log` strips to 8-char hash + date + author + subject — no body, no GPG, no metadata
- `github_pr_list` returns ~80 tokens per PR vs ~2,000 tokens raw from GitHub API
- `github_pr_get` truncates PR body to 500 chars; strips author metadata, reactions, CI detail arrays
- `git_diff` defaults to `--stat` (file-level summary) — full diff only on explicit request

### Key Skills

- Show what changed in the last N commits
- Summarize what a PR does from its diff
- Find all files touched by an author this week
- Detect which commit introduced a bug (`git_blame` + `git_show`)

---

## 2. CI/CD Server (`servers/cicd`)

**Category**: CI/CD Pipeline  
**Language**: Go  
**Status**: Planned  
**Purpose**: Pipeline status, log analysis, and failure triage across GitHub Actions, GitLab CI, Jenkins.

### Exposed Tools

| Tool | Description |
|---|---|
| `cicd_analyze` | Analyze a pipeline/job failure — returns only error context, not full logs |
| `cicd_get` | Get pipeline/run/job status |
| `cicd_list` | List recent runs, failed jobs, flaky tests |
| `cicd_trigger` | Trigger a pipeline or re-run a job |
| `cicd_search` | Search historical runs by status, branch, trigger |

### Token Optimization

- `cicd_analyze` downloads full logs server-side, extracts only: stack traces, error lines, 10 lines of pre-failure context
- Discards: timestamps, build progress lines, successful step outputs, dependency download logs
- Typical reduction: 50,000 token log → 300–500 token failure summary

### Key Skills

- Identify root cause of a failing test (extract assertion diff + file + line)
- Detect flaky tests by analyzing failure frequency across N runs
- Summarize all failures across a PR's CI runs into one triage report

---

## 3. Codebase Indexer (`servers/codebase`)

**Category**: Codebase Indexer  
**Language**: Go  
**Status**: `mcpx-code` Implemented · `mcpx-api` Planned  
**Purpose**: Static analysis, AST navigation, symbol resolution, and API schema parsing — no LLM required for lookups.

### 3a. Code Intelligence (`mcpx-code`)

| Tool | Description | LLM? |
|---|---|---|
| `code_search` | Full-text search across a repo. Returns file:line matches | no |
| `code_find` | Locate a symbol (function, type, var) by name. Returns definition + signature + preview | no |
| `code_callers` | List all call sites of a function | no |
| `code_deps` | Import list for a file or package. Supports Go and Python | no |
| `code_explain` | Explain a file:line range in plain English via Ollama | yes |
| `code_diff_review` | Review a diff for code smells or correctness issues via Ollama | yes |

**Ollama model**: auto-detected from running instance via `/api/ps` — no config needed if a model is already loaded. Override with `OLLAMA_MODEL` in the server's `env:` block. See [mcpx-code.md](servers/mcpx-code.md) for full configuration.

### 3b. API Intelligence (`mcpx-api`)

| Tool | Description |
|---|---|
| `api_list` | List all endpoints in a spec or codebase. Returns method, path, summary, auth |
| `api_get` | Full details of one endpoint: params, request body, response schemas |
| `api_search` | Search endpoints by keyword (path, tag, description) |
| `api_diff` | Compare two spec versions — breaking vs non-breaking changes |
| `api_gen` | Generate curl example, Go client snippet, or mock handler via Ollama |
| `api_validate` | Validate a request/response payload against an endpoint's schema |

### Token Optimization

- Symbol lookups return: signature + 5 lines of body + caller list — not full file contents
- `code_search` replaces 20–40 sequential `grep` + `read` calls with 1–2 tool calls
- `api_list` returns ~50 tokens per endpoint vs ~800 tokens raw from an OpenAPI spec

### Key Skills

- "Where is X defined and who calls it?" → `code_find` + `code_callers`
- "What files change if I modify this interface?" → `code_deps`
- "Did this API release introduce breaking changes?" → `api_diff`
- "Generate a curl example for this endpoint" → `api_gen`

---

## 4. Codebase Intelligence (`servers/debug`)

**Category**: Codebase Intelligence  
**Language**: Go  
**Status**: Planned (see [mcpx-debug](servers/mcpx-debug.md))  
**Purpose**: Runtime and static analysis that requires contextual understanding — uses local Ollama for diagnosis and explanation.

### Exposed Tools

| Tool | Description |
|---|---|
| `debug_error` | Analyze an error + stack trace. Returns: root cause, file:line, suggested fix |
| `debug_logs` | Triage a log snippet — strip noise, surface anomalies |
| `debug_test` | Analyze failing test output — which assertion failed, why, what to change |
| `debug_diff` | Explain what broke in a before/after diff and why |
| `debug_explain` | Explain a specific file+line range in plain terms |

### Token Optimization

- Stack frames from vendor paths, stdlib, and runtime internals are stripped before the Ollama call
- Only ±20 lines around each relevant frame are injected as source context
- Output is structured (cause / location / fix / confidence) — never raw Ollama response

### Key Skills

- "Why is this panic happening?" → `debug_error` with stack trace + repo path
- "What's wrong with this log output?" → `debug_logs`
- "Why did this test start failing after my change?" → `debug_diff`

---

## 5. Exec Sandbox (`servers/exec`)

**Category**: Exec Sandbox  
**Language**: Go  
**Status**: Planned  
**Purpose**: Run code or scripts server-side in isolation and return only the final output.

### Exposed Tools

| Tool | Description |
|---|---|
| `exec_run` | Run a code snippet (Python, JS, Bash, Go) in an isolated environment |
| `exec_query` | Run a database query and return filtered results |
| `exec_script` | Run a multi-step script with access to internal APIs, return final output only |

### Design

- Each execution gets an ephemeral container or seccomp-isolated process
- The LLM writes a script that does all intermediate processing (API calls, data joins, filtering)
- Only the final return value reaches the LLM context — collapses N sequential tool calls into 1 turn

### Security Constraints

- No network access by default (explicit opt-in per request)
- Read-only filesystem except for a `/tmp/workspace` scratch dir
- Execution timeout: 30s default, configurable up to 5 min
- No access to host environment variables or credentials

### Key Skills

- Reconcile data across two APIs (write the join logic as a script)
- Parse a 100MB log archive and return the 5 most relevant errors
- Generate a migration script and dry-run it against a test DB

---

## 6. Infrastructure / K8s Server (`servers/infra`)

**Category**: Infrastructure / K8s  
**Language**: Go  
**Status**: Planned  
**Purpose**: Cloud infrastructure telemetry, Kubernetes operations, alert triage.

### Exposed Tools

| Tool | Description |
|---|---|
| `infra_get` | Get: pod, deployment, service, node, alert, metric |
| `infra_list` | List: pods by namespace, unhealthy nodes, active alerts, recent events |
| `infra_analyze` | Analyze: cluster health, alert root cause, resource pressure |
| `infra_apply` | Apply: scale deployment, restart pod, patch config |
| `infra_search` | Search: events, logs (Loki/CloudWatch), metrics (Prometheus) |

### Token Optimization

- `infra_analyze alert` aggregates across multiple monitoring sources server-side
- Returns: affected service, probable cause, relevant metric window — not raw Prometheus JSON
- Pod list strips managed fields, internal annotations, full container spec; returns name + status + restart count + age

### Key Skills

- "Why is this pod crash-looping?" → `infra_analyze` aggregates events + logs + resource metrics
- "What's consuming the most memory in prod?" → `infra_list` filtered and sorted server-side
- "Scale the api deployment to 5 replicas" → `infra_apply`

---

## 7. Local LLM Router (`servers/llm`)

**Category**: Local LLM Router  
**Language**: Go (Ollama HTTP client) / Python (if native ML libs needed)  
**Status**: Planned (see [mcpx-scribe](servers/mcpx-scribe.md), [mcpx-test](servers/mcpx-test.md))  
**Purpose**: Delegate generation tasks to a local Ollama model — keeping token-heavy generation out of the cloud LLM context.

### 7a. Documentation Generation (`mcpx-scribe`)

| Tool | Description |
|---|---|
| `scribe_generate` | Generate a doc comment for a function, type, or file |
| `scribe_update` | Detect and rewrite stale doc comments in a file. Returns a diff |
| `scribe_readme` | Generate or update a README section from source code |
| `scribe_search` | Search documentation across a repo by natural-language query |
| `scribe_coverage` | Report which exported symbols in a package lack doc comments |

### 7b. Test Generation (`mcpx-test`)

| Tool | Description |
|---|---|
| `test_generate` | Generate unit tests for a function or file, matching repo style |
| `test_gaps` | Identify untested branches and edge cases — returns descriptions, not code |
| `test_analyze` | Parse test failure output and explain what failed and why |
| `test_coverage` | Parse a coverage report and surface highest-value uncovered lines |
| `test_mock` | Generate mock/stub code for an interface or dependency |

### Token Optimization

- Ollama prompts are kept under 1,200 tokens — only the target symbol is sent, not the full file
- `scribe_update` uses `git blame` to detect staleness before calling Ollama — avoids unnecessary LLM calls
- `test_gaps` uses static branch analysis first; Ollama only called for complex logic (>50 branches)
- Generated code is returned raw (no markdown fences, no explanation) — ready to write to disk

### Key Skills

- Generate doc comments for all undocumented exports in a package
- Detect and rewrite doc comments that no longer match the implementation
- Generate table-driven tests matching the repo's existing style
- Identify the highest-value untested code paths without running the test suite

---

## 8. System Server (`servers/system`)

**Category**: System  
**Language**: Go  
**Status**: Planned  
**Purpose**: Direct host access — directory tree navigation and shell command execution on the local machine. Not sandboxed. For trusted developer use, not for running LLM-generated code (that belongs in the exec sandbox).

### Exec Sandbox vs. System Server

| | Exec Sandbox | System Server |
|---|---|---|
| **Trust level** | Untrusted (LLM-written code) | Trusted (developer-directed) |
| **Isolation** | Ephemeral container / seccomp | Direct host process |
| **Filesystem** | `/tmp/workspace` only | Configurable allowed paths |
| **Network** | Blocked by default | Host network |
| **Use case** | Agent runs arbitrary scripts | Agent runs `make`, `go test`, navigates repo |

### Exposed Tools

| Tool | Description |
|---|---|
| `sys_tree` | List directory tree with depth/filter control |
| `sys_run` | Run a shell command on the host and return stdout/stderr |
| `sys_read` | Read a file with line range support |
| `sys_write` | Write or append to a file |
| `sys_find` | Find files by name pattern or content grep |
| `sys_env` | Read environment variables (allowlisted keys only) |

### Security Constraints

- Command allowlist in `system.yaml` (default: `go`, `make`, `git`, `docker`, `kubectl`, `grep`, `find`, `cat`, `ls`, `curl`, `jq`)
- Blocked patterns: `rm -rf`, `sudo`, `chmod 777`, pipe to shell (`| sh`, `| bash`)
- Working directory must be within configured `allowed_paths`
- Output truncated at 8,000 characters
- Hard timeout: 120s maximum

---

## 9. Search & Discovery Meta-Tool (built into proxy)

Not a separate server — implemented in the proxy gateway.

| Tool | Description |
|---|---|
| `search_tools` | Semantic search over all available tools across all servers |

On session start, only `search_tools` is injected into the LLM context. When the agent calls it with a natural language query, the proxy finds the top 2–3 matching tools and returns their full schemas for that turn only.

This single tool replaces the 17,000+ tokens of upfront schema loading that a naive MCP setup would consume.

---

## Summary

| Server | Category | Language | Status |
|---|---|---|---|
| `mcpx-git` | Git / PR Management | Go | **Implemented** |
| `mcpx-cicd` | CI/CD Pipeline | Go | Planned |
| `mcpx-code` | Codebase Indexer | Go | **Implemented** |
| `mcpx-api` | Codebase Indexer | Go | Planned |
| `mcpx-debug` | Codebase Intelligence | Go | Planned |
| `mcpx-exec` | Exec Sandbox | Go | Planned |
| `mcpx-infra` | Infrastructure / K8s | Go | Planned |
| `mcpx-scribe` | Local LLM Router | Go | Planned |
| `mcpx-test` | Local LLM Router | Go | Planned |
| `mcpx-system` | System | Go | Planned |

---

## Future / External

| Server | Purpose |
|---|---|
| `servers/docs` | Search internal wikis, Notion, Confluence, README files |
| `servers/secrets` | Vault / AWS Secrets Manager lookup (read-only) |
| `servers/db` | Schema inspection, read-only queries, migration status |
| `servers/notify` | Send Slack messages, create tickets (Jira/Linear) |

External-server-backed skills (filesystem, browser, Postgres, Slack, Sentry, etc.) are documented in [external-skills.md](external-skills.md) — those don't require building a new server.
