# MCP Servers

Active servers shipped by default and optional servers available when needed.

---

## Active (default build)

| Server | Description | Status |
|---|---|---|
| [mcpx-exec](mcpx-exec.md) | Test/build output filtering + code snippet execution | **Active** |
| [mcpx-debug](mcpx-debug.md) | Stacktrace/panic/log triage — extracts errors, filters noise | **Active** |

---

## Optional (add to gateway.yaml when needed)

| Server | Description | Status |
|---|---|---|
| mcpx-git | Local git CLI + GitHub PR API | Available |
| mcpx-code | Symbol search, dependency graph, code explanation via Ollama | Available |

To enable an optional server, add it to `gateway.yaml`:

```yaml
- name: git
  transport: stdio
  binary: mcpx-git
```

---

## Planned

| Server | Description | Value |
|---|---|---|
| mcpx-cicd | CI/CD log filtering — GitHub Actions, GitLab CI, Jenkins | High — daily CI failures generate 2K–10K token logs |
| mcpx-scribe | Doc comment generation + staleness detection via Ollama | Medium — batch doc writing sessions |
| mcpx-test | Test generation, coverage gap analysis via Ollama | Medium — occasional use |

---

## Port assignments (HTTP mode)

| Server | Default port |
|---|---|
| mcpx-proxy | 8080 (admin: 9090) |
| mcpx-exec | 8089 |
| mcpx-debug | 8087 |
| mcpx-git | 8081 |
| mcpx-code | 8083 |
