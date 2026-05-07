# mcpx-code

**Category: Codebase Indexer**

Code intelligence server. Provides symbol search, AST-level navigation, dependency graph, and semantic code explanation — without loading the entire codebase into the LLM context.

## Domain

Static analysis and code understanding:
- Symbol and definition lookup (functions, types, constants)
- Dependency and import graph
- AST-level structural search (find all callers of a function)
- Semantic explanation of a code block via local LLM

Designed to replace the pattern of dumping 10 files into Claude's context to answer "where is X defined?".

## Tools

| Tool | Description | LLM? |
|---|---|---|
| `code_search` | Full-text search across a codebase. Returns file:line matches. | no |
| `code_find` | Locate a symbol (function, type, var) by name. Returns definition site + signature + preview. | no |
| `code_callers` | Find all call sites of a function. Returns file:line:context for each. | no |
| `code_deps` | Show import list for a file or package directory. Supports Go and Python. | no |
| `code_explain` | Explain a file:line range in plain English. | yes (Ollama) |
| `code_diff_review` | Review a unified diff for correctness issues and code smells. | yes (Ollama) |

Total: 6 tools. `code_search`, `code_find`, `code_callers`, `code_deps` are pure static analysis — zero LLM calls, zero added latency.

## Data flow

```
caller → code_find(symbol="Gateway", repo="/path/to/repo")
           │
           ├─ walk repo (skips .git, vendor, node_modules, ...)
           ├─ extract symbol table per file (language auto-detected)
           │     Go:     go/ast stdlib — full AST
           │     Python: regex on top-level def/class
           │     JS/TS:  regex on function/class/const declarations
           ├─ rank: exact name match first, then partial
           └─ return LEAN table: symbol · file:line · signature · 5-line preview

caller → code_explain(repo, path, start_line, end_line)
           │
           ├─ read lines [start, end] from file
           ├─ if OLLAMA_MODEL not set → GET /api/ps → use first loaded model
           └─ POST /api/generate → Ollama → 2-4 sentence plain-English explanation
```

## Language support

| Language | Symbol extraction | Caller search | Import graph |
|---|---|---|---|
| Go | `go/ast` stdlib (full) | yes | yes |
| Python | regex + indentation | yes | yes |
| TypeScript/JS | regex | yes | no |
| Other | grep fallback (text only) | yes | no |

## Index strategy

No persistent index. Indexes on demand per tool call. Repo walk skips `.git`, `vendor`, `node_modules`, `__pycache__`, `dist`, `build`, `.next`, `target`. Results capped at 20 by default (configurable via `limit` param).

## Ollama integration

Only `code_explain` and `code_diff_review` call Ollama.

### Model resolution (in order)

1. `OLLAMA_MODEL` env var — if set, always used
2. `GET /api/ps` — uses the first model currently loaded in Ollama (cached after first call)
3. Error — if Ollama is unreachable or no model is loaded

This means **no configuration is required** if you already have a model running:
```bash
ollama run gemma4:e4b-it-bf16   # model auto-detected on first call
```

### Environment variables

All optional. Set via `env:` block in `gateway.yaml`.

| Variable | Default | Description |
|---|---|---|
| `OLLAMA_URL` | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | _(auto-detect)_ | Model name; omit to use whatever is loaded |
| `OLLAMA_NUM_CTX` | `16384` | Context window size (`131072` for Gemma 4) |
| `OLLAMA_TEMPERATURE` | _(Ollama default)_ | 0.0–1.0; lower = more deterministic |
| `OLLAMA_TOP_P` | _(Ollama default)_ | Nucleus sampling threshold |
| `OLLAMA_TOP_K` | _(Ollama default)_ | Token candidate pool size |

### Compatible models (Ollama)

| Model | Pull command | Notes |
|---|---|---|
| `qwen2.5-coder:7b` | `ollama pull qwen2.5-coder:7b` | Good default for code tasks |
| `qwen2.5-coder:14b` | `ollama pull qwen2.5-coder:14b` | Higher quality, ~8 GB |
| `gemma4:e4b-it-bf16` | `ollama pull gemma4:e4b-it-bf16` | Google Gemma 4, 128K context |
| `deepseek-coder:6.7b` | `ollama pull deepseek-coder:6.7b` | Strong on Python/TS |
| `codellama:7b` | `ollama pull codellama:7b` | Meta, general code |

## Configuration (gateway.yaml)

Minimal — no `env` block needed if Ollama is running locally with a model loaded:

```yaml
- name: code
  transport: stdio
  binary: mcpx-code
```

Full example with overrides:

```yaml
- name: code
  transport: stdio
  binary: mcpx-code
  disabled: true              # optional: start disabled, enable from admin /ui
  env:
    OLLAMA_MODEL: gemma4:e4b-it-bf16
    OLLAMA_NUM_CTX: "131072"
    OLLAMA_TEMPERATURE: "0.2"
```

The `disabled: true` flag starts the server in disabled state. It can be toggled live from the admin dashboard at `http://localhost:9090/ui`.

## Output format

`code_find` — LEAN key-value:
```
symbol  Gateway
file    proxy/internal/gateway/gateway.go:17
sig     type Gateway struct{...}
preview type Gateway struct {
          cfg       *config.Config
          reg       *registry.Registry
```

`code_search` / `code_callers` — one match per line:
```
proxy/internal/gateway/gateway.go:87  if err := gw.Init(ctx); err != nil {
proxy/cmd/main.go:43                  if err := gw.Init(context.Background()); err != nil {
```

`code_deps` — one import per line:
```
context
fmt
github.com/mark3labs/mcp-go/mcp
github.com/nqhuy44/mcpx/proxy/internal/config
```

## Binary

`mcpx-code` — pure Go, uses stdlib `go/ast` for Go parsing.  
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8083`).

## Status

**Implemented** — binary `mcpx-code`, port 8083.
