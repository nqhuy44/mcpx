# mcpx-debug

**Category: Codebase Intelligence**

Debug assistant server. Collects runtime context (stack traces, logs, error messages, relevant source), formats it into a token-lean prompt, calls a local Ollama model, and returns an actionable diagnosis — not raw output.

## Domain

Error triage and root-cause analysis across:
- Panics and stack traces (Go, Python, Node.js)
- Structured and unstructured log files
- Test failure output
- Runtime error messages with source context

## Tools

| Tool | Description |
|---|---|
| `debug_error` | Analyze an error message + optional stack trace. Returns: root cause, affected file:line, suggested fix. |
| `debug_logs` | Triage a log snippet. Strips noise, surfaces anomalies (errors, warnings, latency spikes). |
| `debug_test` | Analyze a failing test output. Returns: which assertion failed, why, and what to change. |
| `debug_diff` | Given a before/after diff, explain what broke and why. |
| `debug_explain` | Explain a specific file+line range in plain terms. Uses Ollama only; no external calls. |

Total: 5 tools.

## Data flow

```
caller → debug_error(message, stack, repo_path?)
           │
           ├─ if repo_path: read relevant source files (±20 lines around each frame)
           ├─ strip duplicate frames, vendor paths, runtime internals
           ├─ format compact prompt (LEAN format, <1500 tokens)
           │
           └─ POST /api/generate → Ollama
                │
                └─ extract structured response:
                     cause: <one line>
                     location: file:line
                     fix: <one line>
                     confidence: high|medium|low
```

## Ollama integration

Calls `POST http://localhost:11434/api/generate` (configurable via `OLLAMA_URL`).

Config via env:
```
OLLAMA_URL      http://localhost:11434   # Ollama base URL
OLLAMA_MODEL    qwen2.5-coder:7b        # model to use
OLLAMA_NUM_CTX  8192                    # context window
```

Model is not hardcoded — any Ollama model works. Recommended: `qwen2.5-coder:7b` (fast, code-aware).

## Source context injection

When `repo_path` is provided, `debug_error` resolves each stack frame to a file and reads ±20 lines around the error site. This is injected into the prompt before the Ollama call. Files outside `repo_path` (stdlib, vendor) are skipped.

## Output format

All tools return plain text optimized for Claude's context window:

```
cause:    nil pointer dereference — cfg.Servers[i] accessed before Init()
location: proxy/internal/gateway/gateway.go:84
fix:      call gw.Init(ctx) before iterating cfg.Servers
confidence: high
```

Never returns raw Ollama JSON or log dumps.

## Binary

`mcpx-debug` — pure Go, no cgo, no Python runtime.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8082`).

## Status

**Planned** — not yet implemented.
