# Architecture

## System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                         MCP HOST (Agent)                             │
│             Claude Code / Cursor / Custom Agent                      │
│                                                                      │
│   Skills layer (.claude/commands/ or orchestrator server)            │
│   review-pr · debug-pipeline · release-notes · security-audit ...   │
└────────────────────────────┬─────────────────────────────────────────┘
                             │  JSON-RPC 2.0
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      mcpx Proxy Gateway                              │
│  • Auth & rate limiting                                              │
│  • Tool registry & semantic search (lazy loading)                   │
│  • Routes tool calls to domain servers                               │
│  • Emits metrics to admin service                                    │
└────┬──────────┬──────────┬──────────┬──────────┬──────────┬─────────┘
     │          │          │          │          │          │
     ▼          ▼          ▼          ▼          ▼          ▼
  [git]     [cicd]    [codebase]  [exec]   [infra]    [llm]
  server    server     server    server    server     server
     │          │          │          │          │       │
     ▼          ▼          ▼          ▼          ▼       ▼
  GitHub    Jenkins/    AST/Deps   Sandbox   K8s/Cloud  Ollama/
  GitLab    GH Actions  Index      Runtime   APIs       llama.cpp

                             ║  metrics channel
                             ▼
              ┌──────────────────────────────┐
              │     mcpx Admin Service       │
              │  • Server health dashboard   │
              │  • Token usage & budgets     │
              │  • Tool call history         │
              │  • Live SSE updates          │
              │  → http://localhost:9090     │
              └──────────────────────────────┘
```

## Communication Protocols

| Mode | Transport | Use Case |
|---|---|---|
| Local dev | stdio (pipe) | Single developer, native binary |
| Team/cloud | HTTP + SSE | Shared server, persistent connections |
| Local LLM | stdio or HTTP | Ollama/vLLM running on same machine or LAN |

The proxy gateway normalizes both transports inward, so domain servers don't need to care about how the client connects.

## Core Components

### 1. Proxy Gateway (Go)

The single entrypoint for all MCP clients. Responsibilities:
- Expose a `search_tools(query: string)` meta-tool for lazy schema loading
- Maintain a registry mapping generic verbs → domain server + resource type
- Forward tool calls to the correct domain server via stdio or HTTP
- Handle auth tokens, per-server rate limits, and error normalization

The gateway itself exposes **≤12 tools** to the LLM, regardless of how many underlying capabilities exist.

### 2. Domain MCP Servers

Independent binaries/services, each owning one area of the SDLC. They communicate with the gateway only — never directly with the LLM host. Each server:
- Implements the MCP spec (JSON-RPC 2.0 over stdio or SSE)
- Performs all heavy processing server-side before returning data
- Returns data in LEAN/TOON format (not raw JSON)
- Exposes its own sub-registry to the gateway during handshake

### 3. Local LLM Router (optional layer)

A lightweight MCP server wrapping a local inference engine (Ollama or llama.cpp). It acts as an intelligent pre-filter:
- Intercepts prompts that don't need cloud intelligence
- Handles: log summarization, code search, formatting, test generation
- Returns results back through MCP as if it were any other tool

See [local-models.md](local-models.md) for hardware-specific guidance.

## Multi-Agent Routing

```
User task
    │
    ▼
Cloud LLM (Claude) ──orchestrates──► complex reasoning, cross-file arch
    │
    ├──routes via MCP──► Local Qwen 3.6 ──► log summarization
    │                                   ──► code search
    │                                   ──► test scaffolding
    │
    └──routes via MCP──► Domain servers ──► git ops, CI/CD, exec sandbox
```

The cloud LLM never does work a local model can do. High-volume, bounded, deterministic tasks go local. Only overarching reasoning and integration stays in the expensive cloud context.

## Deployment Topologies

### Local Developer (stdio)
```
Claude Code
    │ stdio
    ▼
mcpx-proxy (native binary, Go)
    │ stdio
    ├── git-server (native binary)
    ├── codebase-server (native binary)
    └── exec-server (native binary)
```

No Docker. No daemon. Zero infra setup. The proxy binary is the only process the developer configures.

### Team / Self-Hosted (Docker + HTTP/SSE)
```
Developer machine
    │ HTTP/SSE
    ▼
mcpx-proxy (Docker, internal network)
    │ HTTP
    ├── git-server (Docker)
    ├── cicd-server (Docker)
    └── infra-server (Docker)
         │
         └── VPC / private APIs
```

Proxy sits behind an auth layer. Domain servers live in the same private network as internal data stores.

### Bare-Metal DevOps (Systemd)
Used when a domain server needs direct host access (e.g., exec-server managing containers, infra-server touching host networking). Systemd supervises each binary, logs to journald.

```ini
# /etc/systemd/system/mcpx-exec.service
[Unit]
Description=mcpx exec server
After=network.target

[Service]
ExecStart=/usr/local/bin/mcpx-exec-server
Restart=always
User=mcpx

[Install]
WantedBy=multi-user.target
```

## Tool Discovery Flow (Lazy Loading)

```
Session start
    │
    ▼
Gateway injects search_tools() only into context
    │
Agent calls search_tools("analyze failed pipeline")
    │
    ▼
Gateway semantic search over internal registry
    │
    ▼
Returns 2-3 relevant tool schemas to agent
    │
Agent calls specific tool (e.g., cicd_analyze_failure)
    │
    ▼
Gateway routes to cicd-server
    │
    ▼
cicd-server downloads logs, filters to error context, returns LEAN payload
    │
    ▼
~300 tokens reach LLM context  (vs. 50,000 raw)
```

## Schema Registry Structure

Each domain server reports its resource types to the gateway on startup. The gateway maintains a flat registry:

```
verb: "analyze"  resource: "pipeline_failure"  → cicd-server
verb: "get"      resource: "pr_diff"            → git-server
verb: "search"   resource: "codebase_symbol"    → codebase-server
verb: "run"      resource: "code_snippet"       → exec-server
verb: "get"      resource: "k8s_pod_status"     → infra-server
```

The LLM only ever sees generic verbs. The routing intelligence lives in the registry, not in the prompt.
