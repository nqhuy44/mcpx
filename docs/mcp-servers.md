# MCP Servers & Skills

Each server is an independent binary following the registry-based dispatch pattern. The proxy gateway routes to them — they never talk directly to the LLM host.

**Rule**: Every server exposes ≤12 tool names, regardless of how many underlying operations it supports.

---

## 1. Git Server (`servers/git`)

**Language**: Go  
**Purpose**: All version control operations — GitHub, GitLab, Gitea.

### Exposed Tools (Generic Verbs)

| Tool | Description |
|---|---|
| `git_get` | Get a resource: pr, commit, branch, file, diff, review_comment |
| `git_list` | List resources: prs, commits, branches, files_changed, reviews |
| `git_create` | Create: pr, branch, comment, review |
| `git_update` | Update: pr status, labels, assignees |
| `git_search` | Search: code, commits, issues, prs |

### Token Optimization

- PR responses strip: author metadata, commit arrays, CI status details, emoji reactions
- Return only: title, description, consolidated diff (max 200 lines), inline comments with line refs
- `git_get pr` returns ~400 tokens vs. ~8,000 tokens raw from GitHub API
- `git_list commits` returns hash + subject + author only — no full message body, no GPG data

### Key Skills

- Draft PR description from diff
- Find all files changed by a given author in the last N days
- Summarize what a PR does (condensed diff → LEAN payload)
- Detect merge conflicts preemptively

---

## 2. CI/CD Server (`servers/cicd`)

**Language**: Go  
**Purpose**: Pipeline status, log analysis, failure triage across GitHub Actions, Jenkins, GitLab CI.

### Exposed Tools

| Tool | Description |
|---|---|
| `cicd_analyze` | Analyze a pipeline/job failure — returns only error context |
| `cicd_get` | Get pipeline/run/job status |
| `cicd_list` | List recent runs, failed jobs, flaky tests |
| `cicd_trigger` | Trigger a pipeline or re-run a job |
| `cicd_search` | Search historical runs by status, branch, trigger |

### Token Optimization

- `cicd_analyze` downloads full logs server-side, extracts only: stack traces, error lines, 10 lines of pre-failure context
- Discards: timestamps, build progress lines, successful step outputs, dependency download logs
- Typical reduction: 50,000 token log → 300-500 token failure summary

### Key Skills

- Identify root cause of a failing test (extract assertion diff + file + line)
- Detect flaky tests by analyzing failure frequency across N runs
- Summarize all failures across a PR's CI runs into one triage report

---

## 3. Codebase Indexer (`servers/codebase`)

**Language**: Go  
**Purpose**: Semantic navigation of source code. Replaces blind `grep`/`find` loops.

### Exposed Tools

| Tool | Description |
|---|---|
| `code_search` | Semantic search: find symbol, function, type, usage by name or natural language |
| `code_get` | Get a specific symbol with its full context (signature, imports, callers, callees) |
| `code_list` | List: files in a module, all implementations of an interface, all usages of a function |
| `code_graph` | Get dependency graph for a symbol, file, or module |
| `code_index` | Trigger re-indexing of a path |

### How It Works

- Builds AST-based index on first run, incremental updates on file change
- Stores: symbol → file, line, type, imports, callers, callees
- `code_search` uses embedding similarity over symbol names and docstrings (local embedding model, not cloud)
- Returns structured graph payload, not raw file contents

### Token Optimization

- Never returns full file contents unless explicitly asked via `code_get file`
- Function lookup returns: signature + 5 lines of body + list of callers + list of dependencies
- Replaces 20-40 sequential `grep` + `read` tool calls with 1-2 `code_search` + `code_get` calls

### Key Skills

- "Where is X defined and who calls it?" → single `code_get` call
- "What files would I need to change to modify this interface?" → `code_graph`
- "Find all usages of this deprecated function" → `code_search`

---

## 4. Exec Sandbox (`servers/exec`)

**Language**: Go  
**Purpose**: Run code or scripts server-side and return only the final output.

### Exposed Tools

| Tool | Description |
|---|---|
| `exec_run` | Run a code snippet (Python, JS, Bash, Go) in an isolated environment |
| `exec_query` | Run a database query and return filtered results |
| `exec_script` | Run a multi-step script with access to internal APIs, return final output only |

### Design

- Each execution gets an ephemeral container or seccomp-isolated process
- The LLM writes a script that does all intermediate processing (API calls, data joins, filtering)
- Only the final return value reaches the LLM context
- Collapses N sequential tool calls into 1 LLM turn

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

## 5. Infrastructure / K8s Server (`servers/infra`)

**Language**: Go  
**Purpose**: Cloud infrastructure telemetry, Kubernetes operations, alert triage.

### Exposed Tools

| Tool | Description |
|---|---|
| `infra_get` | Get: pod, deployment, service, node, alert, metric |
| `infra_list` | List: pods by namespace, unhealthy nodes, active alerts, recent events |
| `infra_analyze` | Analyze: cluster health, alert root cause, resource pressure |
| `infra_apply` | Apply: scale deployment, restart pod, patch config |
| `infra_search` | Search: events, logs (via Loki/CloudWatch), metrics (via Prometheus) |

### Token Optimization

- `infra_analyze alert` aggregates across multiple monitoring workspaces server-side
- Returns: affected service, probable cause, relevant metric window — not raw Prometheus JSON
- Pod list strips: managed fields, internal annotations, full container spec; returns name + status + restart count + age

### Key Skills

- "Why is this pod crash-looping?" → `infra_analyze` aggregates events + logs + resource metrics
- "What's consuming the most memory in prod?" → `infra_list` filtered and sorted server-side
- "Scale the api deployment to 5 replicas" → `infra_apply`

---

## 6. Local LLM Router (`servers/llm`)

**Language**: Python  
**Purpose**: Wrap a local inference engine (Ollama/llama.cpp) as an MCP server. Enables the cloud model to delegate subtasks to a free local model.

### Exposed Tools

| Tool | Description |
|---|---|
| `llm_summarize` | Summarize a large text block locally |
| `llm_generate` | Generate code, tests, or scripts locally |
| `llm_embed` | Generate embeddings for semantic search |
| `llm_classify` | Classify intent, error type, or sentiment |
| `llm_extract` | Extract structured fields from unstructured text |

### Design

- Backed by Ollama HTTP API (local) or llama.cpp server
- Model selection is per-tool based on task type (see [local-models.md](local-models.md))
- The cloud LLM calls these tools the same way it calls any other MCP tool
- Responses are formatted as LEAN before being returned to the cloud context

### Key Skills

- Summarize a 10,000-line log file without using cloud tokens
- Generate 50 unit tests for a module locally
- Produce embeddings for codebase-server's semantic index

---

## 7. System Server (`servers/system`)

**Language**: Go
**Purpose**: Direct host access — directory tree navigation and shell command execution on the local machine. Not sandboxed. This is for trusted developer use, not for running LLM-generated code (that belongs in the exec sandbox).

### Exec Sandbox vs. System Server

| | Exec Sandbox (`servers/exec`) | System Server (`servers/system`) |
|---|---|---|
| **Trust level** | Untrusted (LLM-written code) | Trusted (developer-directed) |
| **Isolation** | Ephemeral container / seccomp | Direct host process |
| **Filesystem** | `/tmp/workspace` only | Configurable allowed paths |
| **Network** | Blocked by default | Host network |
| **Use case** | Agent runs arbitrary scripts | Agent navigates repo, runs `make`, `go test`, etc. |

### Exposed Tools

| Tool | Description |
|---|---|
| `sys_tree` | List directory tree with depth/filter control |
| `sys_run` | Run a shell command on the host and return stdout/stderr |
| `sys_read` | Read a file (with line range support) |
| `sys_write` | Write or append to a file |
| `sys_find` | Find files by name pattern or content grep |
| `sys_env` | Read environment variables (allowlisted keys only) |

### `sys_tree` — Directory Tree

Returns a compact tree of a directory. Never returns file contents — only structure.

```json
{
  "tool": "sys_tree",
  "arguments": {
    "path": "./servers/git",
    "depth": 3,
    "include": ["*.go", "*.yaml"],
    "exclude": ["vendor", "node_modules", ".git", "*.pb.go"]
  }
}
```

**Output (LEAN)**:
```
@tree path=servers/git depth=3
servers/git/
├── main.go
├── handler/
│   ├── get.go
│   ├── list.go
│   └── create.go
├── registry/
│   └── resources.go
└── config.yaml
files: 6  dirs: 2
```

Token optimization rules:
- Default `depth: 3` — never unlimited recursion
- Default excludes: `vendor/`, `node_modules/`, `.git/`, `dist/`, `*.pb.go`, `*_generated.go`
- File counts replace contents at max depth
- Never list hidden files unless `show_hidden: true` is explicit

### `sys_run` — Shell Command

Runs a single shell command on the host. Returns stdout + stderr + exit code.

```json
{
  "tool": "sys_run",
  "arguments": {
    "cmd": "go test ./... -run TestLEANEncoder -v",
    "cwd": "/home/user/mcpx",
    "timeout_sec": 30
  }
}
```

**Output (LEAN)**:
```
@cmd_result
exit: 0
duration: 2.3s
stdout:
  === RUN   TestLEANEncoder
  --- PASS: TestLEANEncoder (0.00s)
  PASS
  ok   github.com/nqhuy44/mcpx/libs/lean/go  2.341s
stderr: (empty)
```

**Security constraints**:
- Command allowlist configured in `system.yaml` (by default: `go`, `cargo`, `python`, `npm`, `make`, `git`, `docker`, `kubectl`, `grep`, `find`, `cat`, `ls`, `curl`)
- Blocked by default: `rm -rf`, `sudo`, `chmod 777`, pipe to shell (`| sh`, `| bash`)
- Working directory must be within configured `allowed_paths`
- Hard timeout: 120s maximum regardless of input
- Output truncated at 8,000 characters (tail kept if truncated)

**Configuration (`servers/system/system.yaml`)**:
```yaml
allowed_paths:
  - ~/projects
  - /tmp/mcpx-workspace

allowed_commands:
  - go
  - cargo
  - python3
  - make
  - git
  - docker
  - kubectl
  - npm
  - pnpm
  - grep
  - find
  - cat
  - ls
  - curl
  - jq

blocked_patterns:
  - "rm -rf"
  - "| sh"
  - "| bash"
  - "> /dev/sd"
  - "sudo"

max_output_chars: 8000
default_timeout_sec: 30
max_timeout_sec: 120
```

### `sys_find` — Find Files

Combines `find` + `grep` into a single filtered call. Avoids the agent making multiple sequential shell calls to locate files.

```json
{
  "tool": "sys_find",
  "arguments": {
    "path": ".",
    "name": "*.go",
    "contains": "func.*Encode",
    "exclude_dirs": ["vendor", "testdata"]
  }
}
```

**Output (TOON)**:
```
[File|count=4]
path,size,modified
libs/lean/go/encoder.go,3.2KB,2026-05-06
libs/lean/go/encoder_test.go,1.8KB,2026-05-06
libs/toon/go/encoder.go,2.9KB,2026-05-04
proxy/internal/metrics/tokens.go,1.1KB,2026-05-03
```

### Key Skills This Enables

- Agent runs `go build`, `make test`, `cargo check` and sees real output without blind tool chaining
- Agent maps an unfamiliar repo with `sys_tree` before deciding which files to read
- Agent searches for a pattern across files with `sys_find` in one call instead of five `grep` calls
- Agent checks what's running with `sys_run ps aux | grep mcpx` without leaving the MCP interface

---

## 8. Search & Discovery Meta-Tool (built into proxy)

Not a separate server — implemented in the proxy gateway.

| Tool | Description |
|---|---|
| `search_tools` | Semantic search over all available tools across all servers |

On session start, only `search_tools` is injected into the LLM context. When the agent calls it with a natural language query, the proxy embeds the query, finds the top 2-3 matching tools, and returns their full schemas for that turn only.

This single tool replaces the 17,000+ tokens of upfront schema loading that a naive MCP setup would consume.

---

## Planned / Future Servers

| Server | Language | Purpose |
|---|---|---|
| `servers/docs` | Go | Search internal wikis, Notion, Confluence, README files |
| `servers/secrets` | Go | Vault / AWS Secrets Manager lookup (read-only) |
| `servers/db` | Go | Schema inspection, read-only queries, migration status |
| `servers/notify` | Go | Send Slack messages, create tickets (Jira/Linear) |

External-server-backed skills (filesystem, browser, Postgres, Slack, Sentry, etc.) are documented in [external-skills.md](external-skills.md) — those don't require building a server.
