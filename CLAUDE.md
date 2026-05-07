# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`mcpx` is a centralized repository for Model Context Protocol (MCP) servers focused on coding and DevOps automation. The goal is to build a unified platform of domain-specific MCP servers that reduce LLM token consumption, enable local AI routing, and expose a clean interface to agentic workflows like Claude Code.

The project is in pre-implementation phase. See `docs/research.md` for the full architectural research.

## Architecture Principles

### Modular, Domain-Specific Servers
Do **not** build a monolithic MCP server. Each server must address one domain of the software development lifecycle:
- Git/PR management
- CI/CD pipeline analysis
- Kubernetes/infrastructure telemetry
- Codebase indexing and AST resolution
- Sandboxed code execution

### Registry-Based Dispatch (Critical)
Never map every API endpoint to a separate MCP tool — this causes catastrophic schema bloat. Instead:
- Expose a small set of generic verbs (e.g., `get`, `list`, `create`, `analyze`)
- Back them with a declarative registry that routes to specific resources
- Target: ≤12 exposed tools per server, regardless of how many underlying resource types exist
- Reference: Harness MCP redesign dropped context usage from 26% → 1.6% using this pattern

### Token Optimization as First-Class Concern
Every server must minimize what reaches the LLM context window:
- **Lazy-load schemas**: Expose a `search_tools(query)` meta-tool; inject tool schemas only when needed
- **Server-side filtering**: Never expose raw API responses. Pre-process logs, strip metadata, return only actionable fields
- **Code execution sandbox**: Prefer one tool that accepts code (Python/JS) over multi-step sequential calls
- **Serialization format**: Use LEAN or TOON for tabular/list data, YAML for nested config — never pretty-printed JSON

### Proxy/Aggregation Layer
Multiple domain servers should be composable behind a single proxy endpoint. The Claude Code client connects to one proxy that routes tool calls to the appropriate micro-server. This keeps client configuration minimal.

## Technology Stack Decisions

**Default language: Go.** All servers use Go unless there is a specific reason not to.

| Use Case | Language | Reason |
|---|---|---|
| All domain servers (Git, CI/CD, K8s, code exec, etc.) | Go | Default — goroutines, static binary, no runtime deps |
| Orchestration layer / proxy | Go | Same as above |
| LLM/ML pipeline servers | Python + FastAPI | Exception — native AI library access (embeddings, inference) |

Avoid Python for non-LLM servers (GIL, concurrency overhead). Do not use Rust or TypeScript unless there is an extraordinary case — Go covers all non-ML use cases.

### Native Binaries for Local Dev
Go servers compile to static native binaries distributed over stdio. No Docker required for local developer use. Docker is reserved for cloud/enterprise deployments behind a VPC.

## Deployment Topology

- **Local/dev**: Native binary over stdio (zero-infra)
- **Enterprise/team**: Dockerized servers on ECS/Fargate within a VPC, behind an Application Load Balancer
- **Bare-metal DevOps**: Systemd for process supervision when host filesystem/process access is required (avoids Docker-in-Docker)

## Local LLM Routing

The architecture supports a multi-agent router pattern:
- **Orchestrator**: Cloud model (e.g., Claude) handles complex reasoning and cross-file orchestration
- **Local models** (via Ollama/vLLM): Handle repetitive, high-volume tasks — log summarization, script generation, semantic search
- **Preferred local model**: Qwen 3.6 (27B or 35B) — 6× faster than Gemma 4 on equivalent hardware, resilient to quantization, 1M-token context window
- Local models should be packaged as MCP servers themselves, exposing embedding and inference as tools

## Serialization Standards

When returning data to the LLM context:
1. **LEAN format** — best overall (46.7% token reduction vs JSON, highest accuracy)
2. **TOON** — for uniform tabular arrays (30–60% reduction, worse for nested data)
3. **YAML** — for hierarchical/nested config (23.7% reduction)
4. **JSON** — only when the client requires it; never pretty-print

## Key Anti-Patterns to Avoid

- Mapping REST endpoints 1:1 to MCP tools
- Passing raw API responses (especially CI logs, PR metadata) to the LLM
- Loading all tool schemas at session start
- Using Python for servers requiring high concurrency
- Docker-in-Docker when a server needs to manage containers on the host
