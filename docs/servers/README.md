# MCP Servers

All domain servers in mcpx, organized by architecture category.

---

## Git / PR Management

Version control operations and pull request lifecycle.

| Server | Description | Status |
|---|---|---|
| [mcpx-git](mcpx-git.md) | Local git CLI + GitHub PR API | **Implemented** |

---

## CI/CD Pipeline

Pipeline inspection, build analysis, and test result triage.

| Server | Description | Status |
|---|---|---|
| mcpx-cicd | Pipeline runs, failed step logs, test results (GitHub Actions, GitLab CI, Jenkins) | Planned |

---

## Codebase Indexer

Static analysis, AST navigation, symbol resolution — no LLM required for lookups.

| Server | Description | Status |
|---|---|---|
| [mcpx-code](mcpx-code.md) | Symbol search, AST navigation, dependency graph, code explanation via Ollama | **Implemented** |
| [mcpx-api](mcpx-api.md) | OpenAPI/Swagger parsing, endpoint discovery, breaking change diff | Planned |

---

## Exec Sandbox

Safe code execution without exposing shell access.

| Server | Description | Status |
|---|---|---|
| [mcpx-exec](mcpx-exec.md) | Run Python/JS/bash/Go/Ruby/PHP snippets in an isolated temp dir, return stdout + exit code | **Implemented** |

---

## Codebase Intelligence

Runtime and static analysis that requires contextual understanding — uses local LLM for diagnosis.

| Server | Description | Status |
|---|---|---|
| [mcpx-debug](mcpx-debug.md) | Error triage, stack trace analysis, log noise reduction | Planned |

---

## Infrastructure / K8s

Cluster state, resource telemetry, and deployment health.

| Server | Description | Status |
|---|---|---|
| mcpx-infra | Pod status, events, resource usage, Helm release diffs (Kubernetes + cloud) | Planned |

---

## Local LLM Router

Generation tasks delegated to a local Ollama model — keeps generation out of the cloud LLM context.

| Server | Description | Status |
|---|---|---|
| [mcpx-scribe](mcpx-scribe.md) | Doc comment generation, staleness detection, README sections | Planned |
| [mcpx-test](mcpx-test.md) | Unit/integration test generation, coverage gap analysis, mock generation | Planned |

---

## Port assignments (HTTP mode)

| Server | Default port |
|---|---|
| mcpx-proxy | 8080 (admin: 9090) |
| mcpx-git | 8081 |
| mcpx-code | 8083 |
| mcpx-cicd | 8087 |
| mcpx-infra | 8088 |
| mcpx-exec | 8089 |
