# mcpx

MCP proxy for coding automation. A single proxy aggregates domain servers — any MCP client connects to one endpoint and routes tool calls via `call_tool`.

## Architecture

```
Any MCP client (Claude Code, Cursor, Windsurf, Zed, VS Code, ...)
    │
    ▼
mcpx-proxy  (stdio)
    │
    ├── mcpx-exec   (test/build output filtering + code snippet execution)
    └── mcpx-debug  (stacktrace/panic/log triage — extracts errors, filters noise)
```

Each server is a static Go binary over stdio. No Docker required for local use.

The proxy exposes 3 tools to the client: `call_tool`, `search_tools`, `mcpx_guide`.
All domain tool schemas are kept server-side — no schema bloat in the LLM context.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/nqhuy44/mcpx/main/install.sh | bash
```

The script installs binaries to `~/.mcpx/bin/` and auto-configures any detected MCP clients.

## Build from source

```bash
# Build all active servers + proxy
make build

# Single component
make build SERVER=exec
make build SERVER=debug
make build SERVER=proxy

# Cross-compile
make build OS=linux ARCH=amd64
```

Binaries are written to `bin/`:

```
bin/
  mcpx-proxy     # aggregation proxy
  mcpx-exec      # test/build runner + code snippet execution
  mcpx-debug     # stacktrace/panic/log triage
  gateway.yaml   # proxy config
```

## Register with any MCP client

### Claude Code
```bash
claude mcp add mcpx /absolute/path/to/bin/mcpx-proxy
```

### Other clients (Cursor, Windsurf, Zed, VS Code)

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/absolute/path/to/bin/mcpx-proxy"
    }
  }
}
```

| Client | Config file |
|---|---|
| Cursor | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |
| Zed | `~/.config/zed/settings.json` |
| VS Code | `~/.config/Code/User/settings.json` (Linux) · `~/Library/Application Support/Code/User/settings.json` (macOS) |

## Available Tools

All tools are called via `call_tool(name="<tool>", args={...})`.

### Code Execution (`mcpx-exec`)

| Tool | Mode | Description |
|---|---|---|
| `exec_run` | `cmd` + `workdir` | Run a project command (`go test ./...`, `make build`) with output filtering |
| `exec_run` | `code` + `lang` | Run a code snippet in an isolated temp dir |
| `exec_langs` | — | List available runtimes |

**`filter` parameter:**
- `filter=test` — strips passing tests, returns only failures + `N passed, M failed` summary
- `filter=build` — strips warnings/notes, returns only error lines
- `filter=none` — raw output capped at 8 KB (default)

Supported snippet languages: `python`, `javascript`/`node`, `bash`, `go`, `ruby`, `php`.

### Error Triage (`mcpx-debug`)

| Tool | Description |
|---|---|
| `debug_analyze` | Analyze stacktrace, panic, log file, or stderr — filters noise, returns actionable summary |

**Inputs:** `input` (raw text) or `file` (path). Type and language auto-detected.

**Languages:** Go, Python, Node.js, Java, Rust.

**Log formats:** plain text, JSON (`{"level":"error",...}`), structured key=value (logrus/zap/zerolog).

Typical savings: 50-line Go panic → ~60 tokens (~88%). 500-line log file → ~150 tokens (~97%).

## Slash Commands

```bash
cp -r commands/mcpx ~/.claude/commands/   # global
```

| Command | Example |
|---|---|
| `/mcpx:exec` | `/mcpx:exec run tests in this project` |
| `/mcpx:debug` | `/mcpx:debug analyze this panic output` |

MCP Prompt versions (`/mcpx_exec`, `/mcpx_debug`) work in all clients that support `prompts/list`.

## Configuration

`gateway.yaml` lives next to the proxy binary.

```yaml
transport: stdio
port: 8080
admin_port: 9090

servers:
  - name: exec
    transport: stdio
    binary: mcpx-exec

  # Optional — add when needed:
  # - name: git
  #   transport: stdio
  #   binary: mcpx-git
  - name: debug
    transport: stdio
    binary: mcpx-debug
```

Set `disabled: true` on any server to skip it at startup; toggle live from the admin dashboard.

## Admin Dashboard

`http://localhost:9090/ui` — live call counts, token estimates, error rates, enable/disable per server.

```
GET  /api/status
POST /api/servers/{name}/disable
POST /api/servers/{name}/enable
```

## Development

```bash
make test
make lint
make fmt
```

## Docs

| Doc | Description |
|---|---|
| [docs/client-setup.md](docs/client-setup.md) | Per-client setup: Claude Code, Cursor, Copilot, Windsurf, Zed |
| [docs/skills.md](docs/skills.md) | Skills (slash commands) — exec, debug |
| [docs/servers/mcpx-exec.md](docs/servers/mcpx-exec.md) | Exec server — test/build filtering, sandbox, languages |
| [docs/servers/mcpx-debug.md](docs/servers/mcpx-debug.md) | Debug server — stacktrace/panic/log triage, language support |
| [docs/architecture.md](docs/architecture.md) | Architecture decisions and design principles |
