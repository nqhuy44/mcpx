# Tech Stack

## Language Selection

Go is the default language for all servers. Python is the only exception, used exclusively for servers that require native AI/ML libraries.

| Server | Language | Reason |
|---|---|---|
| Proxy gateway | Go | Native binary, goroutines handle concurrent agent connections |
| Git / PR server | Go | API-heavy, I/O-bound, benefits from goroutine concurrency |
| CI/CD server | Go | High-throughput log processing, fast text filtering |
| Codebase indexer | Go | AST parsing with go-tree-sitter; goroutines for incremental indexing |
| Exec sandbox | Go | Process management + seccomp isolation via syscall package |
| Infra / K8s server | Go | Official client-go library, cloud SDK coverage |
| System server | Go | Host-native, static binary, no runtime deps |
| Local LLM wrapper | Python | Ollama/vLLM Python SDKs, embedding libraries — only exception |

### When NOT to use Python

Any server with >10 concurrent agent sessions. The GIL throttles parallel requests. Python is only justified when the task requires native AI/ML libraries (embeddings, inference engines, transformers).

## Go Stack

```
Language:    Go 1.23+
MCP:         mark3labs/mcp-go v0.44.x  (community SDK)
HTTP:        net/http (stdlib)
Config:      viper
Testing:     testify
Build:       goreleaser (cross-compile to static binary)
```

Target: single static binary with no external dependencies. Deploy by copying the binary.

```go
// Server entrypoint pattern
func main() {
    s := server.NewMCPServer("git-server", "1.0.0")
    s.AddTool(mcp.NewTool("git_get",
        mcp.WithDescription("Get a git resource (pr, commit, branch, diff)"),
        mcp.WithSchema(gitGetSchema),
    ), handleGitGet)
    server.ServeStdio(s)
}
```

## Python Stack

```
Language:    Python 3.12+
MCP:         mcp[cli]  (official Anthropic SDK)
Framework:   FastAPI + uvicorn
Schema:      Pydantic v2
Inference:   ollama-python, vllm
Embeddings:  sentence-transformers, fastembed
Runtime:     uv (package manager), Docker for deployment
```

```python
# FastAPI + MCP pattern
from mcp.server.fastapi import create_mcp_router
from fastapi import FastAPI

app = FastAPI()
app.include_router(create_mcp_router(tools=[...]))
```

Use `uv` for dependency management. Never use `pip` directly in project tooling.

## Serialization Libraries

| Language | LEAN/TOON encoder | YAML |
|---|---|---|
| Go | custom (internal pkg) | `gopkg.in/yaml.v3` |
| Python | custom or `lean-fmt` (if published) | `PyYAML` or `ruamel.yaml` |

LEAN and TOON encoders are implemented as shared internal libraries in this repo. No external dependency.

## Project Layout (Target)

```
mcpx/
├── proxy/              # Go — gateway binary
├── servers/
│   ├── git/            # Go
│   ├── cicd/           # Go
│   ├── codebase/       # Go
│   ├── exec/           # Go
│   ├── infra/          # Go
│   ├── system/         # Go
│   └── llm/            # Python — local model wrapper (only exception)
├── libs/
│   ├── lean/           # LEAN/TOON encoders
│   │   ├── go/
│   │   └── python/
│   └── registry/       # Tool registry schema definitions
├── deploy/
│   ├── docker/
│   └── systemd/
└── docs/
```

## Build & CI Requirements

- All Go servers: `go build -o bin/ ./...` produces static binaries
- Python servers: packaged as Docker images only (no native binary distribution)
- CI must run MCP conformance tests against each server before merge
- JSON schemas for all tools auto-generated from code (never hand-written)
