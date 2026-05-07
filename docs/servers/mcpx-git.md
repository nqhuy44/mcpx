# mcpx-git

**Category: Git / PR Management**

Git and GitHub PR server. Wraps local `git` CLI commands and the GitHub REST API, pre-filtering all output to return only actionable fields — never raw API responses or full log dumps.

## Domain

Version control and pull request management:
- Local repository inspection (status, log, diff, blame, show)
- Branch management (list, create, delete)
- GitHub PR lifecycle (list, review, comment)

## Tools

| Tool | Description |
|---|---|
| `git_status` | Working tree status — staged, unstaged, untracked files. |
| `git_log` | Commit history. Filterable by author, path, date range. Returns hash (8 chars), date, author, subject. |
| `git_diff` | Diff working tree, staged changes, or a commit range. Supports `--stat` summary mode. |
| `git_branch` | List, create, or delete local branches via `action` param. |
| `git_show` | Details of a single commit: message, author, date, changed-file summary. |
| `git_blame` | Annotate file lines with the commit and author that last changed each line. Supports line range. |
| `github_pr_list` | List PRs for a GitHub repo. Returns number, title, author, state, branch, labels, date. |
| `github_pr_get` | Full PR details: body (truncated to 500 chars), diff stats, mergeable status. |
| `github_pr_comment` | Post a comment on a PR or issue. |

Total: 9 tools.

## Data flow

```
caller → git_log(repo, n, author?, path?, since?)
           │
           ├─ exec: git log --pretty=format:"%h %ad %an %s" --date=short [-n N] [filters]
           ├─ parse each line → LogEntry{Hash(8), Date, Author, Subject}
           └─ return compact lines: "a1b2c3d4  2025-01-10  Jane  fix: auth timeout"

caller → github_pr_list(owner, repo, state)
           │
           ├─ GET https://api.github.com/repos/{owner}/{repo}/pulls?state={state}
           ├─ map response → lean PR struct (12 fields, body excluded)
           └─ return formatted lines: "#42 fix: auth [draft] feature→main by:jane 2025-01-10"
```

## Local git operations

All local tools use `exec.Command("git", ...)` — no git library dependency. Binary paths are resolved relative to the config file's directory via `resolveBinary()`.

Key git flags used for token efficiency:
- `git log --pretty=format:"%h %ad %an %s"` — strips verbosity, 8-char hash
- `git diff --stat` — file-level summary before full diff
- `git status --short` — compact format

## GitHub client

Thin custom HTTP client (`internal/github`). No third-party GitHub SDK.

- `GITHUB_TOKEN` env var — optional for public repos, required for private
- Rate limit: unauthenticated 60 req/h, authenticated 5000 req/h
- PR body truncated to 500 chars to limit context usage

## Output format

All output is plain text, token-lean. Example responses:

`git_log`:
```
a1b2c3d4  2025-01-10  Jane Doe     fix: nil pointer in gateway init
f5e6d7c8  2025-01-09  John Smith   feat: add stdio transport
```

`github_pr_list`:
```
#42 fix: auth timeout  feature→main  by:jane  2025-01-10
#41 feat: admin UI [draft]  admin→main  by:john  2025-01-09 labels:wip
```

`git_blame` (line range):
```
a1b2c3d4 2025-01-10 Jane Doe    84: func (g *Gateway) Init(ctx context.Context) error {
a1b2c3d4 2025-01-10 Jane Doe    85:     for _, srv := range g.cfg.Servers {
f5e6d7c8 2025-01-09 John Smith  86:         if err := g.connect(ctx, srv); err != nil {
```

## Configuration

In `gateway.yaml`:
```yaml
- name: git
  transport: stdio
  binary: mcpx-git
```

Environment:
```
GITHUB_TOKEN    GitHub personal access token (optional for public repos)
MCP_TRANSPORT   http | stdio (default: stdio)
MCP_PORT        8081 (used when MCP_TRANSPORT=http)
```

## Binary

`mcpx-git` — pure Go, no cgo, no external dependencies beyond `mcp-go`.
Requires `git` in `$PATH` on the host machine.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8081`).

## Status

**Implemented** — `servers/git/`
