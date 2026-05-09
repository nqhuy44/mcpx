# Skills

Skills are pre-built routing guides registered as MCP Prompts by the proxy. Any client that supports `prompts/list` can invoke them as slash commands.

## How skills work

When a skill is invoked, the proxy injects a routing guide into the LLM context specifying exactly which `call_tool` call to make, in what order, with what parameters.

## Active skills

| Skill | MCP Prompt (all clients) | Claude Code alias |
|---|---|---|
| Test/build runner | `/mcpx_exec <request>` | `/mcpx:exec <request>` |
| Error triage | `/mcpx_debug <request>` | `/mcpx:debug <request>` |

## `mcpx_exec`

Runs project test suites or builds with output filtering, or executes code snippets.

**Test run:**
```
/mcpx_exec run tests
→ call_tool(name="exec_run", args={"cmd":"go test ./...","workdir":"<project>","filter":"test"})
→ returns: "3 passed, 1 failed\n\nFAIL: TestBar\n    bar_test.go:15: expected 42, got 43"
```

**Build:**
```
/mcpx_exec build the project
→ call_tool(name="exec_run", args={"cmd":"make build","workdir":"<project>","filter":"build"})
→ returns: only error lines, warnings stripped
```

**Code snippet:**
```
/mcpx_exec run this python snippet
→ call_tool(name="exec_run", args={"code":"...","lang":"python"})
```

## `mcpx_debug`

Analyzes a stacktrace, panic output, log blob, or stderr — returns condensed summary.

**Stacktrace:**
```
/mcpx_debug analyze this Go panic
→ call_tool(name="debug_analyze", args={"input":"<pasted text>"})
→ returns: "ERROR: nil pointer dereference\n  at handler/api.go:42 → api.Handle\nCAUSE: ..."
```

**Log file:**
```
/mcpx_debug why is the app crashing, check /var/log/app.log
→ call_tool(name="debug_analyze", args={"file":"/var/log/app.log","type":"logs"})
→ returns: "3 error(s) found\n[142] ERROR: connection refused ..."
```

## Skill definitions

Defined in `proxy/internal/gateway/skills.go`. Each skill has `name`, `description`, and `guide`. At runtime the user's `request` argument is appended to the guide before returning.

## Claude Code aliases

The `commands/mcpx/` directory contains identical skill guides as Claude Code slash commands:

```bash
cp -r commands/mcpx ~/.claude/commands/   # global install
```

Enables `/mcpx:exec` and `/mcpx:debug` locally. The MCP Prompt versions (`/mcpx_exec`, `/mcpx_debug`) work in all clients.
