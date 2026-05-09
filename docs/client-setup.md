# Client Setup Guide

How to connect mcpx and use its skills in each MCP client.

## How mcpx teaches clients to use tools

mcpx exposes three mechanisms, in order of preference:

| Mechanism | How it works | Client support |
|---|---|---|
| **MCP Prompts** (`mcpx_usage`) | Auto-injected into system context by the client on session start | Clients that call `prompts/list` on connect |
| **`mcpx_guide` tool** | LLM calls this tool at session start to learn the routing table | Any client — the LLM sees the tool description and calls it |
| **Manual system prompt** | You paste the routing guide into the client's custom instructions | Any client with a system prompt field |

All domain tool schemas stay server-side. The client only receives 3 tool definitions (`call_tool`, `search_tools`, `mcpx_guide`) — ~180 tokens of fixed overhead regardless of how many backend tools exist.

## Skills (slash commands for any client)

mcpx registers skills as MCP Prompts. Any client that supports `prompts/list` can invoke them as slash commands:

| Skill | Invoke | What it does |
|---|---|---|
| `mcpx_exec` | `/mcpx_exec run tests in this project` | Test/build runner with output filtering, code snippets |
| `mcpx_debug` | `/mcpx_debug analyze this panic` | Stacktrace/panic/log triage — extracts errors, filters noise |

Each skill injects a routing guide telling the LLM exactly which `call_tool` call to make.

For Claude Code, identical slash commands are also available via `commands/mcpx/` as `/mcpx:exec` and `/mcpx:debug`.

---

## Claude Code

```bash
# global (all projects)
claude mcp add mcpx ~/.mcpx/bin/mcpx-proxy
```

`.mcp.json` (project root):
```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

Claude Code automatically calls `prompts/list` on connect, so `mcpx_usage` is injected into every session.

Slash commands:
```bash
cp -r commands/mcpx ~/.claude/commands/
```

---

## Cursor

Config file: `~/.cursor/mcp.json`

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

Cursor calls `prompts/list` on connect — `mcpx_usage` is injected automatically.

---

## GitHub Copilot (VS Code)

Config file: `~/.config/Code/User/settings.json` (Linux) or `~/Library/Application Support/Code/User/settings.json` (macOS)

```json
{
  "mcp": {
    "servers": {
      "mcpx": {
        "type": "stdio",
        "command": "/home/you/.mcpx/bin/mcpx-proxy"
      }
    }
  }
}
```

Add to **VS Code → Settings → GitHub Copilot → Custom Instructions**:

```
You have mcpx MCP tools. Call call_tool(name="exec_run", args={"cmd":"<cmd>","workdir":"<path>","filter":"test"}) to run tests with filtered output. Use mcpx_guide to see all available tools.
```

---

## Windsurf

Config file: `~/.codeium/windsurf/mcp_config.json`

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

Add to **Windsurf → Settings → AI Rules**:

```
Call mcpx_guide at session start. Use call_tool(name="exec_run",...) with filter=test or filter=build to run project commands with filtered output.
```

---

## Zed

Config file: `~/.config/zed/settings.json`

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

---

## Google Antigravity

Config file: `~/.gemini/antigravity/mcp_config.json`

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

---

## Any other client

```json
{
  "mcpServers": {
    "mcpx": {
      "command": "/home/you/.mcpx/bin/mcpx-proxy"
    }
  }
}
```

Add to the client's system prompt:

```
You have mcpx MCP tools. Invoke them via call_tool(name="<tool>", args={...}).

Key tools:
- exec_run: run project tests/builds with filtered output
  call_tool(name="exec_run", args={"cmd":"go test ./...","workdir":"/path","filter":"test"})
  filter=test returns only failures. filter=build returns only errors.

Call mcpx_guide to see the full routing table.
```

---

## Verifying the connection

```
call mcpx_guide
```

You should get back a routing table listing all connected servers and their tools.
