# mcpx-scribe

**Category: Local LLM Router**

Documentation server. Generates, updates, and searches documentation using local Ollama — keeping docs in sync with code without touching the LLM's main context window.

## Domain

Documentation lifecycle:
- Generate doc comments from function/type signatures and surrounding code
- Detect stale docs (doc comment no longer matches implementation)
- Search existing docs by semantic query
- Generate or update README sections from code structure

## Tools

| Tool | Description |
|---|---|
| `scribe_generate` | Generate a doc comment for a function, type, or file. Returns the comment text ready to insert. |
| `scribe_update` | Detect and rewrite stale doc comments in a file. Returns a diff of changes. |
| `scribe_readme` | Generate or update a README section (usage, API, config) from source code. |
| `scribe_search` | Search documentation across a repo by natural-language query. Returns ranked file:line matches. |
| `scribe_coverage` | Report which exported symbols in a package lack doc comments. |

Total: 5 tools.

## Data flow

```
caller → scribe_generate(path, symbol, repo_path?)
           │
           ├─ extract target symbol: signature + body (first 30 lines)
           ├─ extract surrounding context: package name, imports, sibling types
           ├─ build compact prompt (<800 tokens):
           │     "Write a concise doc comment for this Go function. Return only the comment."
           │
           └─ POST /api/generate → Ollama → strip markdown → return raw comment text

caller → scribe_update(path, repo_path)
           │
           ├─ parse all exported symbols with existing doc comments
           ├─ for each: compare comment against current signature+body
           ├─ flag mismatches (heuristic: param names changed, return type changed)
           ├─ regenerate flagged comments via Ollama
           └─ return unified diff (caller applies it or sends to Claude for review)
```

## Staleness detection

A doc comment is flagged as stale when:
- A parameter name in the comment doesn't appear in the current signature
- The comment mentions a return value type that no longer matches
- The function body changed significantly (>40% line diff) since the comment was last modified (via `git blame`)

Heuristic, not perfect — the diff is always shown for human review.

## Output format

`scribe_generate` returns only the comment text, no wrapper:
```
// Init connects to all configured downstream MCP servers and verifies
// that each is reachable before returning. Returns the first connection
// error encountered, or nil if all servers are healthy.
```

`scribe_update` returns a unified diff:
```diff
-// Init starts the gateway.
+// Init connects to all configured downstream MCP servers and verifies
+// that each is reachable before returning.
```

`scribe_coverage` returns a LEAN table:
```
package  proxy/internal/gateway
missing  3/12 exported symbols
─────────────────────────────────
Gateway.Shutdown    no doc
RouteResult         no doc
ErrServerTimeout    no doc
```

## Ollama integration

All tools except `scribe_coverage` call Ollama. Prompts are kept under 1000 tokens — the model only sees the target symbol, not the whole file.

```
OLLAMA_URL      http://localhost:11434
OLLAMA_MODEL    qwen2.5-coder:7b
OLLAMA_NUM_CTX  4096
```

## Binary

`mcpx-scribe` — pure Go.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8084`).

## Status

**Planned** — not yet implemented.
