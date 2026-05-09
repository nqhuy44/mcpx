# mcpx-debug

Error triage server. Takes a stacktrace, panic output, log file, or any stderr blob and returns a condensed, actionable summary — errors extracted, noise stripped, fix suggested when pattern matched.

## Tool

### `debug_analyze`

```
call_tool(name="debug_analyze", args={
  "input":   "<raw text>",          // stacktrace, panic, log lines, stderr
  "file":    "/path/to/app.log",    // alternative to input
  "type":    "auto",                // auto|stacktrace|panic|logs|stderr
  "lang":    "auto",                // auto|go|python|node|java|rust
  "context": 3                      // lines of context around each error in log mode
})
```

## Language support

| Language | Detects | Filters |
|---|---|---|
| Go | `panic:`, `runtime error:`, `fatal error:` | Drops runtime/sync/stdlib frames |
| Python | `Traceback (most recent call last)` | Drops stdlib/site-packages frames |
| Node.js | `TypeError:`, `at Object.` | Drops node_modules/node: frames |
| Java | `Exception in thread`, `XxxException:` | Drops java./javax./sun. frames |
| Rust | `thread 'main' panicked` | Drops std::/core::/registry frames |

## Log format support

- Plain text with level keywords (`ERROR`, `FATAL`, `PANIC`, `WARN`)
- JSON logs (`{"level":"error","msg":"..."}`)
- Structured key=value logs (`level=error msg="..."`)
- Logrus, zap, zerolog output
- Python `logging` module format
- Syslog

## Output format

```
type:panic lang:go

ERROR: runtime error: index out of range [5] with length 3
  at internal/handler/request.go:42 → handler.Process
  at cmd/server.go:18 → main.run
  [4 stdlib/runtime frames]

CAUSE: slice/array bounds exceeded — check length before indexing
```

For logs:
```
type:logs lang:unknown

3 error(s) found

[142] ERROR: connection refused (postgres:5432)
    [139]   2024-01-01 12:00:01 INFO starting server
    [143]   2024-01-01 12:00:04 WARN retrying in 5s

CAUSE: service not running or wrong host/port — check connectivity
```

## Token savings

| Input | Raw tokens | After debug_analyze | Reduction |
|---|---|---|---|
| 50-line Go panic | ~500 | ~60 | ~88% |
| 200-line Python traceback | ~2,000 | ~100 | ~95% |
| 500-line log file | ~5,000 | ~150 | ~97% |

## Binary

`mcpx-debug` — pure Go, no cgo, no external runtime deps.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8087`).
