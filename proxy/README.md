# mcpx-proxy

The MCP gateway for mcpx. It is the single endpoint that Claude Code (or any MCP host) connects to. It aggregates all domain servers behind one address, routes tool calls to the right server, and exposes `search_tools` for lazy schema discovery.

## How it works

```
MCP host (Claude Code)
        │ JSON-RPC 2.0
        ▼
  mcpx-proxy
        ├── search_tools (built-in)
        ├── git_get  ──────────────► mcpx-git  (stdio)
        ├── git_list ──────────────► mcpx-git  (stdio)
        ├── cicd_analyze ──────────► mcpx-cicd (HTTP)
        └── ...
```

On startup the proxy connects to each configured domain server, fetches its tool list, and registers every tool as a pass-through handler. The MCP host sees one server with all tools. The proxy routes each call transparently.

`search_tools` is always registered regardless of which domain servers are configured. Call it with a natural language query to find the right tool before calling it — this keeps schema tokens out of context until they are needed.

## Quick start

```bash
go build -o bin/mcpx-proxy ./cmd
./bin/mcpx-proxy gateway.yaml
```

The binary reads `gateway.yaml` from the argument or falls back to `gateway.yaml` in the working directory.

Add it to your Claude Code config (`~/.claude.json` or `.mcp.json`):

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/path/to/bin/mcpx-proxy",
      "args": ["/path/to/gateway.yaml"]
    }
  }
}
```

## Configuration

```yaml
# gateway.yaml
transport: stdio   # how the proxy serves the MCP host: "stdio" (default) or "http"
port: 8080         # used only when transport is "http"

servers:
  - name: git
    transport: stdio
    binary: ../bin/mcpx-git      # path to the domain server binary

  - name: cicd
    transport: http
    address: http://localhost:8081  # base URL of the domain server
```

| Field | Values | Description |
|---|---|---|
| `transport` | `stdio`, `http` | How the proxy itself is served |
| `port` | integer | HTTP listen port (ignored for stdio) |
| `servers[].transport` | `stdio`, `http` | How the proxy connects to that domain server |
| `servers[].binary` | path | Binary to fork (stdio servers only) |
| `servers[].address` | URL | Base address (HTTP servers only) |

## `search_tools`

```
tool: search_tools
input: { "query": "string" }
```

Returns the top-3 matching tools with name, server, description, and input schema. Use this before calling an unfamiliar tool.

Example response:
```
[1] git_get | server:git
  desc: Get a git resource (pr, commit, branch, diff)
  schema: {"type":"object","properties":{"resource":...}}

[2] git_list | server:git
  desc: List git resources (prs, commits, branches)
  schema: ...
```

## Domain server transports

**stdio** — the proxy forks the binary and communicates over stdin/stdout using MCP's JSON-RPC 2.0 framing. The binary must implement the MCP spec. No network port required.

**HTTP** — the proxy connects to a running HTTP server that speaks the MCP Streamable HTTP transport. The server must be already running before the proxy starts.

## Project layout

```
proxy/
├── cmd/main.go                    # entry point
├── internal/
│   ├── config/config.go           # config loading (viper)
│   ├── registry/registry.go       # tool registry with keyword search
│   ├── transport/
│   │   ├── client.go              # Client interface
│   │   ├── stdio.go               # stdio transport
│   │   └── http.go                # HTTP transport
│   └── gateway/gateway.go         # wires config, registry, transport, MCP server
└── gateway.yaml                   # example config
```

## Building

Requires Go 1.23+.

```bash
go build -o bin/mcpx-proxy ./cmd
```

Cross-compile for Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o bin/mcpx-proxy-linux-amd64 ./cmd
```
