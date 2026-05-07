# mcpx

A centralized platform of domain-specific [Model Context Protocol](https://modelcontextprotocol.io) (MCP) servers for coding and DevOps automation. A single proxy aggregates all domain servers — Claude Code connects to one endpoint and routes tool calls automatically.

## Architecture

```
Claude Code
    │
    ▼
mcpx-proxy  (stdio)
    │
    ├── mcpx-git    (git + GitHub PR tools)
    ├── mcpx-cicd   (coming soon)
    └── mcpx-k8s    (coming soon)
```

Each domain server is a static Go binary communicating over stdio. No Docker required for local use.

## Prerequisites

- Go 1.23+
- `git` in your PATH
- Claude Code CLI (`claude`)

## Build

```bash
# Build all components (proxy + all servers)
make build

# Build a single component
make build SERVER=git
make build SERVER=proxy

# Cross-compile
make build OS=linux ARCH=amd64
```

Binaries are written to `bin/`:

```
bin/
  mcpx-proxy     # aggregation proxy
  mcpx-git       # git + GitHub PR server
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

You should see `mcpx` listed as connected with 9 tools.

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

Set `GITHUB_TOKEN` for private repos or to raise API rate limits:

```bash
export GITHUB_TOKEN=ghp_...
```

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

The proxy reads `gateway.yaml` from the same directory as the binary (`bin/gateway.yaml`). To add or disable a server, edit `proxy/gateway.yaml` and rebuild:

```yaml
transport: stdio
port: 8080
admin_port: 9090

servers:
  - name: git
    transport: stdio
    binary: mcpx-git        # resolved relative to gateway.yaml location
  # - name: cicd
  #   transport: http
  #   address: http://localhost:8081
```

An admin dashboard is available at `http://localhost:9090/ui` when the proxy is running.

## Development

```bash
make test          # run tests for all modules
make lint          # go vet + staticcheck
make fmt           # gofmt in-place

make test SERVER=git     # test a single module
```

To run the proxy standalone (after building):

```bash
./bin/mcpx-proxy
```

To add a new domain server, create a `servers/<name>/` directory with its own `go.mod` and `cmd/main.go`. The Makefile picks it up automatically.
