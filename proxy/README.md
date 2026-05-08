# mcpx-proxy

The MCP gateway for mcpx. It is the single endpoint that any MCP client connects to. It aggregates all domain servers behind one address, routes tool calls to the right server, and exposes `search_tools` for lazy schema discovery.

## How it works

```
MCP client (Claude Code, Cursor, Windsurf, Zed, ...)
        │ JSON-RPC 2.0
        ▼
  mcpx-proxy
        ├── search_tools  (built-in)
        ├── git_status  ──────────────► mcpx-git  (stdio)
        ├── git_log     ──────────────► mcpx-git  (stdio)
        ├── code_find   ──────────────► mcpx-code (stdio)
        ├── exec_run    ──────────────► mcpx-exec (stdio)
        └── ...
```

On startup the proxy connects to each configured domain server, fetches its tool list, and registers every tool as a pass-through handler. The MCP client sees one server with all tools. The proxy routes each call transparently.

`search_tools` is always registered regardless of which domain servers are configured. Call it with a natural language query to find the right tool before calling it — this keeps schema tokens out of context until they are needed.

## Quick start

```bash
make build
./bin/mcpx-proxy
```

The binary looks for `gateway.yaml` in the same directory as itself, then falls back to `gateway.yaml` in the working directory.

## Configuration

`gateway.yaml` is the single config file. The Makefile copies it to `bin/gateway.yaml` on every build.

```yaml
transport: stdio      # how the proxy serves the MCP client: "stdio" (default) or "http"
port: 8080            # used only when transport is "http"
admin_port: 9090      # admin dashboard port

servers:
  - name: git
    transport: stdio
    binary: mcpx-git         # resolved relative to gateway.yaml's directory

  - name: code
    transport: stdio
    binary: mcpx-code
    disabled: false          # set true to skip at startup
    env:
      OLLAMA_MODEL: qwen2.5-coder:7b

  - name: exec
    transport: stdio
    binary: mcpx-exec

  - name: cicd            # example HTTP server
    transport: http
    address: http://localhost:8087
```

| Field | Values | Description |
|---|---|---|
| `transport` | `stdio`, `http` | How the proxy itself is served to the MCP client |
| `port` | integer | HTTP listen port (ignored for stdio) |
| `admin_port` | integer | Admin dashboard port (default 9090) |
| `servers[].transport` | `stdio`, `http` | How the proxy connects to that domain server |
| `servers[].binary` | path | Binary to fork (stdio only) — relative to `gateway.yaml` |
| `servers[].address` | URL | Base address (HTTP only) |
| `servers[].disabled` | bool | Skip this server at startup (default false) |
| `servers[].env` | map | Env vars injected into the server subprocess |

## `search_tools`

```
tool: search_tools
input: { "query": "string" }
```

Returns the top-3 matching tools with name, server, description, and input schema. Use this before calling an unfamiliar tool to avoid loading all schemas into context.

Example response:
```
[1] exec_run | server:exec
  desc: Execute a code snippet and return stdout/stderr/exit code.
  schema: {"properties":{"language":...,"code":...,"timeout":...}}

[2] git_diff | server:git
  desc: Show diff for a file, commit range, or staged changes.
  schema: ...
```

## Admin dashboard

Available at `http://localhost:9090/ui`.

- Live call counts and error rates per server
- Per-server status: `ok` · `error` · `connecting` · `disabled`
- Enable/Disable toggle — re-spawns or kills the subprocess live

API:
```
GET  /api/status                 — JSON snapshot of all metrics
POST /api/servers/{name}/disable — disable a server
POST /api/servers/{name}/enable  — re-enable a server
```

## Project layout

```
proxy/
├── cmd/main.go                    # entry point, config loading, transport selection
├── internal/
│   ├── config/config.go           # config loading (viper), binary path resolution
│   ├── registry/registry.go       # tool registry with keyword search (search_tools)
│   ├── transport/
│   │   ├── client.go              # Client interface
│   │   ├── stdio.go               # stdio transport — fork + stdin/stdout MCP
│   │   └── http.go                # HTTP transport — connect to running HTTP server
│   ├── gateway/gateway.go         # wires config, registry, transports, MCP server
│   ├── admin/                     # admin dashboard HTTP handler
│   └── metrics/                   # per-server call/error counters
└── gateway.yaml                   # config (copied to bin/ on make build)
```

## Building

```bash
# via Makefile (recommended — copies gateway.yaml to bin/)
make build SERVER=proxy

# direct
cd proxy && go build -o ../bin/mcpx-proxy ./cmd
```

Cross-compile:
```bash
make build SERVER=proxy OS=linux ARCH=amd64
```
