# mcpx-api

**Category: Codebase Indexer**

API intelligence server. Parses OpenAPI/Swagger specs, discovers endpoints from source code, generates client code and request examples, and diffs API changes — without loading full specs into the LLM context.

## Domain

API development lifecycle:
- Parse and query OpenAPI 3.x / Swagger 2.0 specs
- Discover API endpoints directly from Go/Python router code (no spec required)
- Generate curl examples, HTTP client code, or mock handlers
- Diff two spec versions to surface breaking changes

## Tools

| Tool | Description |
|---|---|
| `api_list` | List all endpoints in a spec or codebase. Returns method, path, summary, auth requirement. |
| `api_get` | Get full details of one endpoint: params, request body schema, response schemas. |
| `api_search` | Search endpoints by keyword (path, tag, description). Returns ranked matches. |
| `api_diff` | Compare two spec versions. Returns breaking changes (removed endpoints, changed params) vs non-breaking. |
| `api_gen` | Generate a curl example, Go HTTP client snippet, or mock handler for an endpoint. Uses Ollama. |
| `api_validate` | Validate a request/response payload against an endpoint's schema. Returns validation errors. |

Total: 6 tools.

## Data flow

```
caller → api_list(spec_path or repo_path)
           │
           ├─ if spec_path: parse OpenAPI YAML/JSON → extract endpoints
           ├─ if repo_path (no spec): walk source, detect router patterns:
           │     Go:     chi/mux/gin/echo route registrations
           │     Python: FastAPI @app.get / Flask @app.route decorators
           │
           └─ return LEAN table: method path summary auth_required

caller → api_diff(spec_a, spec_b)
           │
           ├─ parse both specs
           ├─ compare endpoint sets: added / removed / changed
           ├─ for changed: compare param names, types, required flags; response codes
           └─ classify each change: breaking | non-breaking | deprecated
```

## Source discovery (no spec)

When `repo_path` is provided instead of a spec file, `api_list` walks the source and detects routes from common router libraries:

| Framework | Detection pattern |
|---|---|
| Go chi | `r.Get("/path", handler)` |
| Go gin | `r.GET("/path", handler)` |
| Go echo | `e.GET("/path", handler)` |
| Python FastAPI | `@app.get("/path")` decorator |
| Python Flask | `@app.route("/path")` decorator |

Returns best-effort results — handler name is used as summary when no docstring exists.

## Breaking change classification

`api_diff` classifies changes as:

| Change | Breaking? |
|---|---|
| Endpoint removed | yes |
| Required param added | yes |
| Param type changed | yes |
| Response field removed | yes |
| Optional param added | no |
| Response field added | no |
| Description changed | no |
| Endpoint deprecated | warning |

## Output format

`api_list` returns a LEAN table:
```
method  path                          summary                  auth
GET     /repos/{owner}/{repo}/pulls   List pull requests       bearer
POST    /repos/{owner}/{repo}/pulls   Create pull request      bearer
GET     /repos/{owner}/{repo}/pulls/{number}  Get PR details   bearer
```

`api_diff` returns grouped output:
```
BREAKING (2)
  REMOVED  DELETE /users/{id}
  CHANGED  POST /tokens — param "scope" now required

NON-BREAKING (3)
  ADDED    GET /users/{id}/sessions
  ...
```

`api_gen` returns raw code:
```bash
curl -X POST https://api.example.com/repos/owner/repo/pulls \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"fix: auth","head":"fix-auth","base":"main"}'
```

## Ollama integration

Only `api_gen` calls Ollama. All parsing, search, diff, and validation are pure static analysis.

```
OLLAMA_URL      http://localhost:11434
OLLAMA_MODEL    qwen2.5-coder:7b
OLLAMA_NUM_CTX  4096
```

## Binary

`mcpx-api` — pure Go. OpenAPI parsing via `github.com/getkin/kin-openapi`.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8086`).

## Status

**Planned** — not yet implemented.
