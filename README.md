# mcpx

A centralized platform of domain-specific [Model Context Protocol](https://modelcontextprotocol.io) (MCP) servers for coding and DevOps automation. A single proxy aggregates all domain servers — any MCP client connects to one endpoint and routes tool calls automatically.

## Architecture

```
Any MCP client (Claude Code, Cursor, Windsurf, Zed, VS Code, ...)
    │
    ▼
mcpx-proxy  (stdio)
    │
    ├── mcpx-git    (git + GitHub PR tools)
    ├── mcpx-code   (symbol search, AST nav, dependency graph, code explanation)
    └── mcpx-exec   (sandboxed code execution — python, js, bash, go, ruby, php)
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
make build SERVER=exec
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
  mcpx-exec      # sandboxed code execution
  gateway.yaml   # proxy config (copied from proxy/gateway.yaml)
```

## Register with any MCP client

### Claude Code
```bash
claude mcp add mcpx /absolute/path/to/bin/mcpx-proxy
# verify:
/mcp
```

### Other clients (Cursor, Windsurf, Zed, VS Code)

Add to your client's MCP config file:

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/absolute/path/to/bin/mcpx-proxy"
    }
  }
}
```

Config file locations:

| Client | Config file |
|---|---|
| Cursor | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |
| Zed | `~/.config/zed/settings.json` |
| VS Code | `~/Library/Application Support/Code/User/settings.json` (macOS) |

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

| Tool | Description | Requires Ollama |
|---|---|---|
| `code_search` | Full-text search across a codebase | no |
| `code_find` | Locate a symbol by name — definition + signature + preview | no |
| `code_callers` | Find all call sites of a function | no |
| `code_deps` | Import list for a file or package | no |
| `code_explain` | Explain a file:line range in plain English | yes |
| `code_diff_review` | Review a diff for bugs and code smells | yes |

See [docs/servers/mcpx-code.md](docs/servers/mcpx-code.md) for language support and Ollama setup.

### Code Execution (`mcpx-exec`)

| Tool | Description |
|---|---|
| `exec_run` | Run a code snippet — returns stdout, stderr, exit code |
| `exec_langs` | List runtimes available on this machine |

Supported languages: `python`, `javascript`/`node`, `bash`, `go`, `ruby`, `php`.

Runs in an isolated temp directory with a configurable timeout (1–60s). Output is capped at 8 KB per stream to prevent token flooding.

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
| `/mcpx:exec` | `/mcpx:exec run this python snippet` |

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
    env:
      OLLAMA_MODEL: qwen2.5-coder:7b   # optional — auto-detected if Ollama is running

  - name: exec
    transport: stdio
    binary: mcpx-exec
```

### Per-server `env` block

Any key under `env:` is injected into the server process environment. This is how Ollama config, GitHub tokens, and other per-server settings are passed without polluting the global environment.

### `disabled:` flag

Set `disabled: true` on any server to skip it at startup. Can be toggled live from the admin dashboard without restarting the proxy.

## Admin Dashboard

Available at `http://localhost:9090/ui` when the proxy is running.

- Live call counts, token estimates, error rates per server
- Per-server status: `ok` · `error` · `connecting` · `disabled`
- Enable / Disable toggle — restarts a server subprocess without restarting the proxy

API endpoints:
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
make test SERVER=exec
```

To run the proxy standalone (after building):
```bash
./bin/mcpx-proxy
```

To add a new domain server, create `servers/<name>/` with its own `go.mod` and `cmd/main.go`. The Makefile picks it up automatically.

## Docs

| Doc | Description |
|---|---|
| [docs/servers/mcpx-git.md](docs/servers/mcpx-git.md) | Git server — tools, output format, GitHub auth |
| [docs/servers/mcpx-code.md](docs/servers/mcpx-code.md) | Code server — tools, Ollama setup, language support |
| [docs/servers/mcpx-exec.md](docs/servers/mcpx-exec.md) | Exec server — sandbox, supported languages, limits |
| [docs/servers/README.md](docs/servers/README.md) | All servers index with port assignments |
| [docs/architecture.md](docs/architecture.md) | Architectural decisions and design principles |
