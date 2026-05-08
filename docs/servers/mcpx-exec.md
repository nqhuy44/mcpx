# mcpx-exec

**Category: Exec Sandbox**

Sandboxed code execution server. Runs code snippets in an isolated temp directory and returns stdout, stderr, and exit code — replacing the multi-step "write file → run → read output" pattern with a single tool call.

## Domain

Safe, ephemeral code execution:
- Run a snippet in any supported language without shell access
- Capture stdout and stderr separately, capped to prevent token flooding
- Enforce a hard timeout to prevent runaway processes

## Tools

| Tool | Description |
|---|---|
| `exec_run` | Execute a code snippet. Returns exit code, stdout, stderr. |
| `exec_langs` | List runtimes currently available in PATH on this machine. |

Total: 2 tools.

## `exec_run` parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `language` | string | yes | Runtime: `python`, `javascript`, `bash`, `go`, `ruby`, `php` |
| `code` | string | yes | Source code to execute |
| `stdin` | string | no | Data to pass to the program's stdin |
| `timeout` | number | no | Timeout in seconds, 1–60 (default: 10) |

Language aliases: `python3` = `python`, `js` / `node` = `javascript`, `sh` = `bash`.

## Output format

```
exit:0
---
Hello, world!
```

On failure:
```
exit:1
---
stderr:
NameError: name 'foo' is not defined
```

If output exceeds 8 KB per stream, it is truncated:
```
exit:0
---
line 1
line 2
...
... [4096 bytes truncated]
```

Exit code `124` means the process was killed by the timeout.

## Sandbox model

Each `exec_run` call:
1. Creates a fresh `os.MkdirTemp` directory
2. Writes the code to `script.<ext>` inside that directory
3. Forks the runtime with `exec.CommandContext` (timeout enforced via `context.WithTimeout`)
4. Sets `cmd.Dir` to the temp directory so relative paths are safe
5. Deletes the temp directory on return (`defer os.RemoveAll`)

There is no network isolation at this layer — that is handled by the deployment environment (Docker, VPC, or systemd unit with `PrivateNetwork=true` for enterprise use).

## Supported runtimes

| Language | Runtime | File extension | Default port (HTTP mode) |
|---|---|---|---|
| python / python3 | `python3` | `.py` | — |
| javascript / js / node | `node` | `.js` | — |
| bash / sh | `bash` / `sh` | `.sh` | — |
| go | `go run` | `.go` | — |
| ruby | `ruby` | `.rb` | — |
| php | `php` | `.php` | — |

`exec_langs` returns only runtimes found in `$PATH` — call it first if unsure what is available.

## Configuration (gateway.yaml)

```yaml
- name: exec
  transport: stdio
  binary: mcpx-exec
```

No additional env vars required.

Environment:

```
MCP_TRANSPORT   http | stdio (default: stdio)
MCP_PORT        8089 (used when MCP_TRANSPORT=http)
```

## Limits

| Limit | Value |
|---|---|
| Max timeout | 60 seconds |
| Default timeout | 10 seconds |
| Max stdout | 8 KB (truncated) |
| Max stderr | 8 KB (truncated) |
| Working directory | Fresh temp dir per call, deleted after |

## Binary

`mcpx-exec` — pure Go, no cgo, no external dependencies beyond `mcp-go`.  
Requires the target language runtime in `$PATH` on the host machine.  
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8089`).

## Status

**Implemented** — `servers/exec/`
