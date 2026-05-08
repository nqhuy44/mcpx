# Skills

Skills are pre-built routing guides that tell the LLM exactly which tools to call for a given type of task. They are implemented as MCP Prompts registered by the proxy — any MCP client that supports `prompts/list` can invoke them as slash commands.

## How skills work

When a skill is invoked, the proxy injects a short routing guide into the LLM context. The guide tells the model:
- Which tool(s) to call first
- What order to chain them in
- How to interpret and summarize the results

The LLM does not need to figure out the tool routing itself — the skill does that ahead of time.

## Invoking skills

Skills are registered as MCP Prompts named `mcpx_<name>`. All accept an optional `request` argument.

| Skill | MCP Prompt (all clients) | Claude Code alias |
|---|---|---|
| Git operations | `/mcpx_git <request>` | `/mcpx:git <request>` |
| GitHub PRs | `/mcpx_pr <request>` | `/mcpx:pr <request>` |
| Code search / navigation | `/mcpx_code <request>` | `/mcpx:code <request>` |
| Code execution | `/mcpx_exec <request>` | `/mcpx:exec <request>` |
| Infrastructure | `/mcpx_infra <request>` | `/mcpx:infra <request>` |
| Working tree diff | `/mcpx_diff <request>` | `/mcpx:diff <request>` |
| Git blame | `/mcpx_blame <request>` | `/mcpx:blame <request>` |
| Branch management | `/mcpx_branch <request>` | `/mcpx:branch <request>` |

MCP Prompt invocation works in Claude Code, Cursor, Windsurf, VS Code Copilot, Google Antigravity, and Zed — any client that calls `prompts/list` on connect.

The Claude Code aliases (`/mcpx:*`) come from `commands/mcpx/` in this repo. Copy them to enable:

```bash
# global (all projects)
cp -r commands/mcpx ~/.claude/commands/

# project-local
cp -r commands/mcpx .claude/commands/
```

## Skill definitions

Skills are defined in `proxy/internal/gateway/skills.go`. Each skill has:

| Field | Purpose |
|---|---|
| `name` | Becomes `mcpx_<name>` in the prompt registry |
| `description` | Shown in the client's prompt picker |
| `guide` | The routing instructions injected into context |

At runtime the proxy appends the user's `request` argument to the guide before returning it.

## Available skills

### `mcpx_git` / `/mcpx:git`
Git operations on the current repo. Routes to `git_status`, `git_log`, `git_diff`, `git_show` in the right order depending on the request type.

### `mcpx_pr` / `/mcpx:pr`
GitHub pull request workflows. Routes to `github_pr_list`, `github_pr_get`, `github_pr_comment`. Infers owner/repo from the git remote.

### `mcpx_code` / `/mcpx:code`
Code search and navigation. Routes to `code_find`, `code_search`, `code_callers`, `code_deps`, `code_explain`, `code_diff_review` based on the request phrasing. Handles chaining (find → explain) automatically.

### `mcpx_exec` / `/mcpx:exec`
Execute a code snippet or shell command. Prefers `exec_run` over the native shell for large-output commands (find, grep, ps, df, docker ps) to avoid token flooding.

### `mcpx_infra` / `/mcpx:infra`
Infrastructure queries. Routes to `infra_containers`, `infra_services`, `infra_pods`, `infra_disk`, `infra_processes`, etc. Calls `infra_targets` first when a named VM or cluster is referenced.

### `mcpx_diff` / `/mcpx:diff`
Explain what changed in the working tree or between commits. Calls `git_diff(stat=true)` first, then per-file diffs, then summarizes.

### `mcpx_blame` / `/mcpx:blame`
Annotate specific lines with author, date, and commit context. Calls `git_blame` then `git_show` for interesting commits.

### `mcpx_branch` / `/mcpx:branch`
List, create, or delete branches via `git_branch`. Notes that checkout requires the user to run `git checkout` directly.
