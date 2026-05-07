# mcpx

A centralized platform of domain-specific [Model Context Protocol](https://modelcontextprotocol.io) (MCP) servers for coding and DevOps automation. A single proxy aggregates all domain servers — Claude Code connects to one endpoint and routes tool calls automatically.

## Architecture

```
Claude Code (or any MCP client)
    │
    ▼
mcpx-proxy  (stdio)
    │
    ├── mcpx-git    (git + GitHub PR tools)
    └── mcpx-code   (symbol search, AST nav, dependency graph, code explanation)
```

Each domain server is a static Go binary communicating over stdio. No Docker required for local use.

## Install (pre-built binary)

```bash
curl -sSL https://raw.githubusercontent.com/nqhuy44/mcpx/main/install.sh | bash
```

The script:
- Detects your OS and architecture
- Downloads the latest release from GitHub
- Installs binaries to `~/.mcpx/bin/`
- Auto-configures any detected MCP clients (Claude Code, Cursor, Windsurf, Zed, VS Code, Google Antigravity)
- Prints a manual config snippet for everything else

Set `GITHUB_TOKEN` before running to inject it into the config:
```bash
GITHUB_TOKEN=ghp_... curl -sSL https://raw.githubusercontent.com/nqhuy44/mcpx/main/install.sh | bash
```

## Build from source

### Prerequisites

- Go 1.23+
- `git` in your PATH
- [Ollama](https://ollama.com) (optional — only needed for `code_explain` and `code_diff_review`)

### Build

```bash
# Build all components (proxy + all servers)
make build

# Build a single component
make build SERVER=git
make build SERVER=code
make build SERVER=proxy

# Cross-compile
make build OS=linux ARCH=amd64
```

Binaries are written to `bin/`:

```
bin/
  mcpx-proxy     # aggregation proxy
  mcpx-git       # git + GitHub PR server
  mcpx-code      # codebase indexer + code explanation
  gateway.yaml   # proxy config (copied from proxy/gateway.yaml)
```

## Register with Claude Code

```bash
claude mcp add mcpx /absolute/path/to/bin/mcpx-proxy
```

Verify it connected:

```
/mcp
```

## Available Tools

### Git (`mcpx-git`)

| Tool | Description |
|---|---|
| `git_status` | Working tree status — staged, unstaged, untracked |
| `git_log` | Commit history; filter by author, path, date |
| `git_diff` | Diff working tree, staged, or a commit range |
| `git_branch` | List, create, or delete branches |
| `git_show` | Details of a single commit |
| `git_blame` | Annotate file lines with author + commit |
| `github_pr_list` | List PRs (open/closed/all) |
| `github_pr_get` | PR details: body, diff stats, mergeable status |
| `github_pr_comment` | Post a comment on a PR or issue |

Set `GITHUB_TOKEN` in `gateway.yaml` (or as an env var) for private repos and higher rate limits.

### Code Intelligence (`mcpx-code`)

| Tool | Description |
|---|---|
| `code_search` | Full-text search across a codebase. Returns file:line matches |
| `code_find` | Locate a symbol (function, type, var) by name. Returns definition + signature + preview |
| `code_callers` | Find all call sites of a function |
| `code_deps` | Show import list for a file or package |
| `code_explain` | Explain a file:line range in plain English (requires Ollama) |
| `code_diff_review` | Review a diff for bugs and code smells (requires Ollama) |

`code_search`, `code_find`, `code_callers`, `code_deps` work with no Ollama — pure static analysis. See [docs/servers/mcpx-code.md](docs/servers/mcpx-code.md) for language support and configuration.

## Slash Commands

Pre-built Claude Code slash commands live in `commands/mcpx/`. Copy them into your Claude config to enable `/mcpx:*` shortcuts:

```bash
# Project-level (this repo only)
cp -r commands/mcpx .claude/commands/

# Global (all projects)
cp -r commands/mcpx ~/.claude/commands/
```

| Command | Example |
|---|---|
| `/mcpx:git` | `/mcpx:git show last 5 commits` |
| `/mcpx:pr` | `/mcpx:pr list open PRs` |
| `/mcpx:diff` | `/mcpx:diff what changed vs main` |
| `/mcpx:blame` | `/mcpx:blame who wrote lines 10-30 in main.go` |
| `/mcpx:branch` | `/mcpx:branch list branches` |

## Configuration

The proxy reads `gateway.yaml` from the same directory as the binary (`bin/gateway.yaml`).

```yaml
transport: stdio
port: 8080
admin_port: 9090

servers:
  - name: git
    transport: stdio
    binary: mcpx-git

  - name: code
    transport: stdio
    binary: mcpx-code
    # disabled: true          # start disabled; toggle from admin /ui
    env:
      # All optional — omit entirely to use defaults
      # OLLAMA_MODEL auto-detected from running Ollama instance if not set
      OLLAMA_MODEL: qwen2.5-coder:7b
      # OLLAMA_NUM_CTX: "131072"      # use for Gemma 4 (128K context)
      # OLLAMA_TEMPERATURE: "0.2"
```

### Per-server `env` block

Any key under `env:` is injected into the server process environment, overriding the parent process. This is how Ollama, GitHub tokens, and other server-specific config are set — no need to pollute the global environment.

### `disabled:` flag

Set `disabled: true` on any server in `gateway.yaml` to skip it at startup. Useful when a dependency (Ollama, external API) is not yet available. Disabled servers can be enabled at runtime from the admin dashboard without restarting the proxy.

## Admin Dashboard

Available at `http://localhost:9090/ui` when the proxy is running.

- Live call counts, token estimates, error rates per server
- Per-server status: `ok` · `error` · `connecting` · `disabled`
- **Enable / Disable toggle** — click to disable a server (kills the subprocess) or re-enable it (re-spawns and reconnects) without restarting the proxy

API endpoints (used by the UI, also scriptable):

```
GET  /api/status                    — JSON snapshot of all metrics
POST /api/servers/{name}/disable    — disable a server
POST /api/servers/{name}/enable     — re-enable a server
```

## Development

```bash
make test          # run tests for all modules
make lint          # go vet + staticcheck
make fmt           # gofmt in-place

make test SERVER=git     # test a single module
make test SERVER=code
```

To run the proxy standalone (after building):

```bash
./bin/mcpx-proxy
```

To add a new domain server, create a `servers/<name>/` directory with its own `go.mod` and `cmd/main.go`. The Makefile picks it up automatically.

## Docs

| Doc | Description |
|---|---|
| [docs/servers/mcpx-git.md](docs/servers/mcpx-git.md) | Git server — tools, output format, GitHub auth |
| [docs/servers/mcpx-code.md](docs/servers/mcpx-code.md) | Code server — tools, Ollama setup, language support |
| [docs/servers/README.md](docs/servers/README.md) | All servers index with port assignments |
| [docs/mcp-servers.md](docs/mcp-servers.md) | Full server catalogue with planned servers |
| [docs/architecture.md](docs/architecture.md) | Architectural decisions and design principles |
