# Data Formats

All data returned from MCP servers to the LLM context must be serialized in the most token-efficient format possible. This document defines the formats used in this project and when to apply each.

## Format Hierarchy

Choose the format based on data shape:

```
Data shape                     → Format
─────────────────────────────────────────
Uniform tabular/array data     → TOON
Mixed tabular + text           → LEAN   ← default for most responses
Deeply nested config/trees     → YAML
Raw code or file content       → plaintext (no wrapper)
Client requires JSON contract  → JSON (minimized, never pretty-printed)
```

---

## Benchmark Reference

From a 2026 study of 1,170 LLM calls across frontier models:

| Format | Avg Tokens | Savings vs JSON | Accuracy |
|---|---|---|---|
| JSON (pretty) | 7,401 | baseline | 86.2% |
| YAML | 5,647 | −23.7% | 87.4% |
| LEAN | 3,939 | −46.7% | 87.9% |

LEAN is strictly better than JSON and YAML on both token cost and retrieval accuracy.

---

## LEAN Format

LEAN is a block-based serialization format optimized for LLM tokenization. It avoids JSON's structural noise (braces, quotes, commas on every token boundary) while remaining human-readable.

### Syntax

```
@block_name
key: value
key2: value2
---
@block_name
key: value
```

- Blocks delimited by `@name` header and `---` separator
- Keys and values unquoted unless value contains `:` or newline
- Multiline values use indented continuation lines
- Arrays of same-structure objects → each object is a block

### Example: PR Summary

**Raw GitHub API (JSON)** — ~2,100 tokens:
```json
{
  "id": 1842938,
  "number": 412,
  "title": "Add LEAN serializer to codebase server",
  "body": "This PR introduces...",
  "state": "open",
  "user": { "login": "nqhuy", "id": 9182736, "type": "User", ... },
  "head": { "ref": "feat/lean-serializer", "sha": "a3f9...", ... },
  "base": { "ref": "main", ... },
  "labels": [{ "id": 123, "name": "enhancement", ... }],
  "assignees": [],
  "requested_reviewers": [{ "login": "teammate", ... }],
  "created_at": "2026-05-01T10:00:00Z",
  ...42 more fields
}
```

**LEAN output** — ~180 tokens:
```
@pr
number: 412
title: Add LEAN serializer to codebase server
state: open
author: nqhuy
branch: feat/lean-serializer → main
labels: enhancement
reviewer: teammate
created: 2026-05-01
summary:
  Introduces LEAN format encoding to the codebase server response layer.
  Reduces token cost of symbol lookup responses by ~47%.
```

### Example: CI Failure Analysis

**LEAN output** — ~250 tokens:
```
@pipeline_failure
run: 4821
branch: feat/lean-serializer
trigger: push
failed_job: test-unit

@error
file: internal/lean/encoder_test.go:88
type: assertion_failed
message: expected 3939 tokens, got 4102
context:
  86: result := encoder.Encode(fixture)
  87: tokens := countTokens(result)
  88: assert.Equal(t, 3939, tokens)

@pre_failure_context
  Prior step succeeded: build (22s)
  Prior step succeeded: lint (8s)
  First failure: test-unit at 00:01:14
```

### Example: Kubernetes Pod List

**LEAN output** — ~300 tokens (vs. ~4,500 JSON):
```
@pods namespace=production
name: api-deployment-7d9f8b-xk2p
status: Running  restarts: 0  age: 3d
---
name: api-deployment-7d9f8b-mn4q
status: CrashLoopBackOff  restarts: 14  age: 2h
last_error: OOMKilled
---
name: worker-deployment-5c8a-p9wx
status: Running  restarts: 1  age: 5d
```

---

## TOON Format

TOON (Token-Oriented Object Notation) is a compact tabular format for uniform arrays. Keys are declared once in a header — no repetition per row.

### Syntax

```
[TypeName|count=N]
field1,field2,field3,...
value1,value2,value3
value4,value5,value6
```

- Header declares type name and row count
- Second line: comma-separated field names
- Subsequent lines: comma-separated values (no quotes unless value contains comma)
- For values with commas: wrap in `"double quotes"`

### Example: Container Status List

**JSON** — 890 tokens for 5 containers:
```json
[
  { "container_id": "a3f9b2", "name": "api", "status": "running", "uptime": "3d", "cpu": "12%", "mem": "340MB" },
  { "container_id": "b8e1c4", "name": "worker", "status": "running", "uptime": "5d", "cpu": "4%", "mem": "120MB" },
  ...
]
```

**TOON** — ~120 tokens:
```
[Container|count=5]
id,name,status,uptime,cpu,mem
a3f9b2,api,running,3d,12%,340MB
b8e1c4,worker,running,5d,4%,120MB
c2a7d1,db-proxy,running,12d,2%,88MB
d5f0e8,scheduler,stopped,—,—,—
e1b3f6,cache,running,8d,1%,64MB
```

### When TOON Breaks Down

TOON degrades when rows have varying fields or deeply nested values. Switch to LEAN when:
- More than 20% of rows have missing/null fields
- Any field value is itself a nested object
- The "table" has fewer than 3 rows (overhead not worth it)

---

## YAML

Use YAML for hierarchical configuration data, IaC responses, or deeply nested structures where TOON's tabular form doesn't apply and LEAN's block structure becomes awkward.

```yaml
# Good YAML use case: K8s resource spec returned by infra_get
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-deployment
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    spec:
      containers:
        - name: api
          image: registry/api:v2.4.1
          resources:
            requests: { cpu: 100m, memory: 256Mi }
            limits: { cpu: 500m, memory: 512Mi }
```

Avoid YAML for flat lists or tabular data — TOON is more efficient there.

---

## JSON (Fallback Only)

Use JSON only when:
1. The downstream client explicitly requires JSON (e.g., `exec_run` returning a value that feeds into another JSON tool)
2. The tool contract specifies JSON output for machine consumption

**Always minimize**: never pretty-print. Strip all optional whitespace.

```json
{"status":"ok","count":3,"ids":["a1","b2","c3"]}
```

Never:
```json
{
  "status": "ok",
  "count": 3,
  "ids": [
    "a1",
    "b2",
    "c3"
  ]
}
```

---

## Plaintext

For file contents, code snippets, stack traces, and any content where structure adds no value — return as raw plaintext. No wrapper format. The LLM reads plain code better than code embedded in JSON strings (no escape sequences, no `\n` tokens).

```
# Good: code returned as plaintext
func (e *Encoder) Encode(v any) ([]byte, error) {
    buf := &bytes.Buffer{}
    e.write(buf, v, 0)
    return buf.Bytes(), nil
}

# Bad: code wrapped in JSON
{"content": "func (e *Encoder) Encode(v any) ([]byte, error) {\n    buf := &bytes.Buffer{}\n    e.write(buf, v, 0)\n    return buf.Bytes(), nil\n}"}
```

---

## Implementation Notes

### Current State

Implemented servers (`mcpx-git`, `mcpx-code`) use hand-written compact text formatters per tool — no shared encoding library yet. The outputs follow TOON principles for lists and LEAN principles for single objects, but are not produced by a formal encoder.

| Tool | Shape | Effective format |
|---|---|---|
| `git_log`, `code_search`, `code_callers`, `github_pr_list` | Uniform list | TOON-style (ordered fields, no key repetition per row) |
| `github_pr_get`, `code_find` | Single object | LEAN-style (compact key:value, one concept per line) |
| `git_diff`, `code_explain`, `code_diff_review` | Raw text / prose | Plaintext |

### Planned: Shared Encoding Library

A `internal/format` package is planned to centralize LEAN and TOON encoding so all servers use a consistent, tested serializer:

```go
// planned — not yet implemented
import "github.com/nqhuy44/mcpx/internal/format"

// LEAN single object
out := format.LEAN(map[string]any{
    "number": 42, "title": "fix: auth", "state": "open",
})

// TOON uniform list
out := format.TOON(headers, rows)
```

Until that library exists, prefer the decision tree below when writing new tool handlers.

### Format Selection Helper

```
Is the data a uniform list of same-shaped objects?
  Yes → TOON (if >3 rows), else LEAN blocks
  No  → Is it deeply nested (3+ levels)?
          Yes → YAML
          No  → LEAN blocks
```

If in doubt, use LEAN. It is safe for all data shapes and consistently outperforms JSON on both token count and LLM accuracy.
