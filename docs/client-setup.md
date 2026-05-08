# Client Setup Guide

How to connect mcpx and use its skills in each MCP client.

## How mcpx teaches clients to use tools

mcpx exposes three mechanisms, in order of preference:

| Mechanism | How it works | Client support |
|---|---|---|
| **MCP Prompts** (`mcpx_usage`) | Auto-injected into system context by the client on session start | Clients that call `prompts/list` on connect |
| **`mcpx_guide` tool** | LLM calls this tool at session start to learn the routing table | Any client — the LLM sees the tool description and calls it |
| **Manual system prompt** | You paste the routing guide into the client's custom instructions | Any client with a system prompt field |

Most clients only need the MCP server registered — the LLM will discover `mcpx_guide` from the tool list and call it. The manual system prompt is a fallback for clients where you want guaranteed proactive behaviour.

## Skills (slash commands for any client)

mcpx registers per-server skills as MCP Prompts. Any client that supports `prompts/list` can invoke them as slash commands:

| Skill | Invoke | What it does |
|---|---|---|
| `mcpx_git` | `/mcpx_git show last 5 commits` | Git operations — commits, diffs, blame, log |
| `mcpx_pr` | `/mcpx_pr list open PRs` | GitHub PR list, review, comment |
| `mcpx_code` | `/mcpx_code find function parseConfig` | Symbol search, callers, deps, explain |
| `mcpx_exec` | `/mcpx_exec run this python snippet` | Code execution, large-output shell commands |
| `mcpx_infra` | `/mcpx_infra why is api crashing on prod-vm` | Containers, services, Kubernetes, disk |
| `mcpx_diff` | `/mcpx_diff what changed vs main` | Working tree and commit diffs |
| `mcpx_blame` | `/mcpx_blame who wrote lines 10-30 in main.go` | Annotate file lines with author and commit |
| `mcpx_branch` | `/mcpx_branch list branches` | List, create, delete branches |

Each skill injects a routing guide into the LLM context telling it exactly which tool to call and in what order — the LLM does not need to figure this out itself.

For Claude Code specifically, identical slash commands are also available in `commands/mcpx/` using the `/mcpx:<name>` syntax (e.g. `/mcpx:git`). Both work; the MCP Prompt versions work in all clients.

---

## Claude Code

```bash
# global (all projects)
claude mcp add mcpx ~/.mcpx/bin/mcpx-proxy

# or project-local via .mcp.json
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

Claude Code automatically calls `prompts/list` on connect, so `mcpx_usage` is injected into every session. No manual system prompt needed.

Slash commands — copy `commands/mcpx/` to activate `/mcpx:git`, `/mcpx:exec`, etc.:
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

For guaranteed proactive use, also add to **Cursor → Settings → Rules for AI**:

```
You have mcpx tools available. Call mcpx_guide once at the start of any coding session to learn which tool to use for git, code search, and code execution tasks. Prefer tools over asking me to run commands.
```

---

## GitHub Copilot (VS Code)

Config file: `~/Library/Application Support/Code/User/settings.json` (macOS)  
or `~/.config/Code/User/settings.json` (Linux)

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

Add to **VS Code → Settings → GitHub Copilot → Custom Instructions** (or `.github/copilot-instructions.md` in the repo):

```
You have mcpx MCP tools available. At the start of a session call mcpx_guide to learn the routing table. Use git_* tools for any git question, code_* for symbol lookup, exec_run to run code snippets. Never ask the user to run a command if a tool can do it.
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
Call mcpx_guide at session start. Use mcpx tools proactively for git, code search, and code execution instead of asking me to run commands.
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

Add to your Antigravity custom instructions:

```
Call mcpx_guide at session start. Use mcpx tools proactively for git, code search, and code execution instead of asking me to run commands.
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

## Any other client

Add the MCP server using the standard `mcpServers` format with `command` pointing to `mcpx-proxy`.

Then paste this into the client's custom instructions / system prompt field:

```
You have access to mcpx MCP tools for software development tasks.

At the start of each session, call mcpx_guide to get a routing table showing when to use each tool.

Short routing rules:
- git commits / branches / diffs / PRs → git_* and github_pr_* tools
- find a symbol / understand code / trace callers → code_* tools
- run or test a code snippet → exec_run
- unsure which tool → search_tools(query)

Prefer tools over asking the user to run commands manually.
```

---

## Verifying the connection

In any client, ask the LLM:
```
call mcpx_guide
```

You should get back a routing table listing all connected servers and their tools. If you see it, the server is connected and the LLM knows how to use it.
