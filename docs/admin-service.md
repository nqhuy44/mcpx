# Admin Service (`admin/`)

A lightweight management companion to the proxy gateway. It provides a web dashboard to monitor MCP server health, track token usage, inspect tool call history, and set token budgets — all without touching the agent or MCP servers directly.

---

## What It Does

| Feature | Description |
|---|---|
| **Server health** | Live status for every registered MCP server (active, idle, error, unreachable) |
| **Token usage** | Input + output token estimates per server, per tool, per session |
| **Call log** | Recent tool calls: tool name, latency, tokens, status, error message |
| **Token budget** | Set a daily/hourly token cap per server or globally; alert when exceeded |
| **Model routing** | See which calls went to cloud vs. local model |
| **Top tools** | Which tools are called most; which cost the most tokens |

---

## Architecture

The proxy gateway is the single point where all MCP traffic flows. The admin service attaches to the proxy's internal metrics stream — it doesn't intercept live traffic; it receives a copy.

```
MCP Host (Claude Code)
    │ JSON-RPC
    ▼
┌──────────────────────────────────┐
│         Proxy Gateway (Go)       │
│                                  │
│  ┌──────────────────────────┐    │
│  │   Metrics Emitter        │    │
│  │  (per tool call event)   │    │
│  └────────────┬─────────────┘    │
└───────────────│──────────────────┘
                │ in-process channel
                │ (or Unix socket if separate process)
                ▼
┌──────────────────────────────────┐
│       Admin Service (Go)         │
│                                  │
│  SQLite ◄── metrics writer       │
│  HTTP server                     │
│    ├── /api/...  (JSON REST)     │
│    ├── /events   (SSE stream)    │
│    └── /         (Web UI)        │
└──────────────────────────────────┘
         │
         ▼ browser
  Dashboard (HTMX + vanilla CSS)
```

The admin service is a **separate binary** but runs on the same machine as the proxy. They share an in-process channel when bundled together, or communicate over a local Unix socket when run separately.

### Why a separate binary

- Proxy can crash-restart without losing admin state (SQLite persists)
- Admin UI can be deployed selectively (disable it on production servers)
- Keeps proxy binary minimal and fast

---

## Tech Stack

| Layer | Choice | Reason |
|---|---|---|
| Backend | Go | Same language as proxy, static binary, embeds assets |
| Storage | SQLite (`modernc.org/sqlite`) | Zero-dependency, single file, good enough for one proxy |
| UI framework | HTMX + Alpine.js | No build step, templates embedded in binary, SSE support built-in |
| CSS | Pico CSS (classless) | Clean defaults, dark mode, no class soup |
| Charts | Chart.js (CDN, lazy) | Token usage over time; no server rendering needed |
| Real-time | Server-Sent Events (SSE) | Simpler than WebSocket for one-way live updates |

All frontend assets (HTML templates, CSS, JS) are embedded into the Go binary via `embed.FS`. The admin service ships as a single self-contained binary — no npm, no static file serving setup.

---

## Token Counting

Since the proxy sees every tool call, it can estimate tokens for both input and output payloads before forwarding them.

```
Input tokens  = tokens(tool_name) + tokens(schema) + tokens(arguments)
Output tokens = tokens(response_payload)
```

Token estimation uses a Go port of the tiktoken `cl100k_base` encoder — the same BPE model Claude and GPT-4 use. This gives ±5% accuracy without a network call.

```go
// internal/metrics/tokens.go
func EstimateTokens(text string) int {
    // cl100k_base BPE approximation
    // Fast path: char_count / 3.8 (good enough for monitoring)
    // Exact path: run actual BPE encoder (2-3x slower, use for budget enforcement)
    return len(text) / 4  // fast approximation
}
```

For **budget enforcement** (blocking calls that would exceed quota), use the exact BPE path. For **display-only** dashboard numbers, the fast approximation is fine.

---

## Database Schema

```sql
-- MCP servers registered with the proxy
CREATE TABLE servers (
    id          TEXT PRIMARY KEY,  -- e.g. "git-server"
    name        TEXT NOT NULL,
    transport   TEXT NOT NULL,     -- "stdio" | "http"
    endpoint    TEXT,              -- null for stdio
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Health check results (written every 30s)
CREATE TABLE health_checks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   TEXT NOT NULL REFERENCES servers(id),
    status      TEXT NOT NULL,     -- "ok" | "timeout" | "error"
    latency_ms  INTEGER,
    error_msg   TEXT,
    checked_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- One row per tool call
CREATE TABLE tool_calls (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL,
    server_id     TEXT NOT NULL REFERENCES servers(id),
    tool_name     TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    latency_ms    INTEGER NOT NULL,
    status        TEXT NOT NULL,   -- "ok" | "error"
    error_msg     TEXT,
    model_route   TEXT,            -- "cloud" | "local" | null
    called_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tool_calls_server  ON tool_calls(server_id, called_at);
CREATE INDEX idx_tool_calls_session ON tool_calls(session_id, called_at);
CREATE INDEX idx_health_server      ON health_checks(server_id, checked_at);
```

SQLite file stored at `~/.mcpx/admin.db` by default (configurable via `MCPX_ADMIN_DB`).

---

## REST API

All responses are JSON (the admin API is machine-readable; the UI calls this API via HTMX).

```
GET  /api/servers                     — list all servers + current status
GET  /api/servers/:id                 — single server detail
GET  /api/servers/:id/health          — health check history (last 24h)

GET  /api/tools/calls?limit=50        — recent tool calls (all servers)
GET  /api/tools/calls?server=git      — filtered by server
GET  /api/tools/calls?session=xyz     — filtered by session

GET  /api/tokens/summary              — total in/out tokens today, this week
GET  /api/tokens/by-server            — token breakdown per server
GET  /api/tokens/by-tool              — token breakdown per tool name
GET  /api/tokens/timeseries?window=1h — tokens over time (for chart)

GET  /api/budget                      — current budget config
PUT  /api/budget                      — update token budget
POST /api/servers/:id/restart         — restart a stdio-mode server

GET  /events                          — SSE stream of live events
```

---

## SSE Event Types

The `/events` endpoint streams newline-delimited SSE events. The UI subscribes once and gets live updates.

```
event: server_status
data: {"server":"git-server","status":"ok","latency_ms":12}

event: tool_call
data: {"server":"cicd-server","tool":"cicd_analyze","input_tokens":45,"output_tokens":280,"latency_ms":340,"status":"ok"}

event: budget_warning
data: {"server":"git-server","used_tokens":18400,"budget_tokens":20000,"pct":92}

event: server_down
data: {"server":"codebase-server","error":"stdio process exited with code 1"}
```

---

## Web UI Layout

Single-page app with no client-side routing. HTMX polls `/api` endpoints and swaps DOM fragments. SSE stream drives live indicators.

```
┌────────────────────────────────────────────────────────────────────┐
│  mcpx admin                           ● 5 servers  ◉ 3 active      │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  SERVERS                                                           │
│  ┌──────────────┬────────┬──────────┬──────────┬────────────────┐  │
│  │ Server       │ Status │ Calls/h  │ Tokens↑  │ Tokens↓        │  │
│  ├──────────────┼────────┼──────────┼──────────┼────────────────┤  │
│  │ git-server   │ ● ok   │ 142      │ 8.4K     │ 31.2K          │  │
│  │ cicd-server  │ ● ok   │ 23       │ 1.1K     │ 6.8K           │  │
│  │ codebase     │ ● ok   │ 89       │ 4.2K     │ 14.1K          │  │
│  │ exec-server  │ ○ idle │ 4        │ 0.2K     │ 0.9K           │  │
│  │ llm-server   │ ✕ err  │ —        │ —        │ —              │  │
│  └──────────────┴────────┴──────────┴──────────┴────────────────┘  │
│                                                                    │
│  TOKEN USAGE TODAY                   BUDGET: 80,000 tokens         │
│  ████████████████████░░░░  52,400 / 80,000  (65%)                 │
│                                                                    │
│  [Token usage chart — 24h bar chart by server]                    │
│                                                                    │
│  RECENT CALLS                                         [→ full log] │
│  ┌────────────────┬──────────────────┬──────┬─────┬──────────┐    │
│  │ Time           │ Tool             │  In  │ Out │ Latency  │    │
│  ├────────────────┼──────────────────┼──────┼─────┼──────────┤    │
│  │ 14:32:01       │ git_get pr       │   42 │ 180 │  210ms   │    │
│  │ 14:31:58       │ code_search      │   61 │ 340 │  890ms   │    │
│  │ 14:31:44       │ cicd_analyze     │   38 │ 290 │ 1240ms   │    │
│  │ 14:31:30       │ llm_summarize    │  820 │ 140 │  430ms   │    │
│  └────────────────┴──────────────────┴──────┴─────┴──────────┘    │
└────────────────────────────────────────────────────────────────────┘
```

### Server Detail View (click a row)

```
git-server                                     ● active since 09:14
──────────────────────────────────────────────────────────────────
Transport: stdio   Binary: /usr/local/bin/mcpx-git-server
Uptime: 5h 18m    Total calls: 847    Errors: 2 (0.2%)

TOKEN USAGE (today)
  Total in:  18,420   Total out: 74,100
  Top tools:
    git_get       →  in 8.1K   out 42K
    git_list      →  in 4.3K   out 18K
    git_search    →  in 6.0K   out 14K

HEALTH (last 2h, every 30s)  ●●●●●●●●●●●●●●●●●●●●●●●●○●●●●●●
  Avg latency: 18ms   P99: 42ms   Timeouts: 1

RECENT ERRORS
  14:08:22  git_create branch  →  403 Forbidden (missing write token)
  11:30:01  git_get pr/9999    →  404 Not Found
```

---

## Configuration

Admin service config lives in `~/.mcpx/admin.yaml` (or `MCPX_ADMIN_*` env vars):

```yaml
admin:
  listen: "127.0.0.1:9090"    # UI + API address
  db: "~/.mcpx/admin.db"
  health_check_interval: 30s

budget:
  global_daily_tokens: 500000
  alert_threshold: 0.80        # warn at 80%
  per_server:
    git-server: 100000
    llm-server: 200000         # local model, less strict

retention:
  tool_calls_days: 7
  health_checks_days: 2
```

---

## Running the Admin Service

```bash
# Alongside the proxy (recommended — shares in-process metrics channel)
mcpx start --admin

# Standalone (connects to proxy via Unix socket)
mcpx-admin --proxy-socket /tmp/mcpx.sock

# Default dashboard URL
open http://localhost:9090
```

The proxy emits a metrics event on the channel for every:
- Tool call (start + end)
- Server register / deregister
- Health check result
- Budget threshold crossed

---

## Project Layout

```
admin/
├── main.go
├── server/
│   ├── server.go          # HTTP server setup
│   ├── api.go             # REST handlers
│   ├── sse.go             # SSE stream handler
│   └── budget.go          # Budget enforcement logic
├── db/
│   ├── db.go              # SQLite init + migrations
│   ├── queries.go         # Read queries
│   └── writer.go          # Metrics writer (consumes channel)
├── metrics/
│   ├── collector.go       # Receives events from proxy
│   └── tokens.go          # Token estimation (BPE approximation)
└── ui/
    ├── templates/
    │   ├── base.html
    │   ├── dashboard.html
    │   ├── server-detail.html
    │   └── call-log.html
    └── static/
        ├── pico.min.css
        ├── chart.min.js
        └── app.js          # Alpine.js + HTMX init + SSE listener
```

`ui/` is embedded into the binary at build time:

```go
//go:embed ui/templates ui/static
var uiAssets embed.FS
```

No npm. No bundler. The entire admin service — backend + frontend — ships as one static Go binary.
