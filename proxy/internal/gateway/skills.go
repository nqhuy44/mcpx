package gateway

// skill defines one invokable MCP Prompt — a usage guide for a specific server.
// Each skill is registered as a prompt named "mcpx_<name>" with an optional
// "request" argument. Any MCP client that supports prompts/list can invoke them
// as slash commands (e.g. /mcpx_git, /mcpx_infra).
type skill struct {
	name        string
	description string // shown in client prompt pickers
	guide       string // instruction body; "request" argument is appended at runtime
}

func skills() []skill {
	return []skill{
		{
			name:        "git",
			description: "Run a git operation on the current repo: commits, branches, diffs, blame, show",
			guide: `Use the git_* MCP tools to complete this. Follow this order:
1. Call git_status first if the request is about current state or "what changed"
2. Call git_log for history/author/timeline questions (use n=10 by default)
3. Call git_diff for content of changes (use stat=true first, then full diff only if needed)
4. Call git_show for a specific commit hash

Keep the response concise — summarize what you found, don't dump raw output.`,
		},
		{
			name:        "pr",
			description: "Manage or review a GitHub pull request: list, review, comment",
			guide: `Use the github_pr_* MCP tools. Common workflows:

- "list open PRs" → github_pr_list(state=open)
- "review PR 42" → github_pr_get(number=42), summarize title/body/changed files/additions/deletions
- "comment on PR 42: <message>" → github_pr_comment(number=42, body=<message>)
- "show my PRs" → github_pr_list, filter by author name from the results

For reviews: highlight key changes, flag large diffs (>500 lines), note if draft.
Always infer owner/repo from the current git remote — run git_status if unsure.`,
		},
		{
			name:        "code",
			description: "Search, navigate, or explain code in the codebase",
			guide: `Use the code_* MCP tools. Pick the right tool for the request:

- "find function/type/const X" → code_find(symbol="X")
- "search for 'auth token'" → code_search(query="auth token") — use path= to narrow to a dir
- "who calls X" / "where is X used" → code_callers(symbol="X")
- "what does file X import" / "dependencies of X" → code_deps(path="X")
- "explain lines N–M of file X" → code_explain(file="X", start_line=N, end_line=M)
- "review this diff" → code_diff_review(diff="<diff text>")

Chaining:
- For "find and explain X": code_find first to get the file:line, then code_explain on that range
- For "trace how X reaches Y": code_callers(X) to find callers, repeat up the chain (max 3 hops)
- For "what would break if I change X": code_callers(X) + code_deps to map blast radius

Rules:
- code_explain and code_diff_review require Ollama — if they error, tell the user to run: ollama run qwen2.5-coder:7b
- Keep explanations concise — summarize what the code does and why, don't re-read every line back
- For large search results (>20 hits), group by file and summarize rather than listing all matches`,
		},
		{
			name:        "exec",
			description: "Execute a code snippet or shell command and return the output",
			guide: `Use exec_run — not the native Bash tool — for:
- Code snippets in any language (python, javascript, bash, go, ruby, php)
- Shell commands with potentially large output: find, grep, cat large files, ps, df, docker ps

Rules:
- Call exec_langs first if the language is unclear
- Default timeout 10s; raise to 30s for I/O-heavy or package-installing code
- Pass stdin if the code reads from stdin
- On non-zero exit: show stderr, explain the error, suggest a fix
- Summarize large outputs — don't dump raw lines (exec_run caps at 8KB anyway)
- Exception: use the native shell tool for commands that need the project directory (make, git, go build) — exec_run runs in an isolated temp dir`,
		},
		{
			name:        "infra",
			description: "Query infrastructure: Docker containers, systemd services, Kubernetes, disk, processes",
			guide: `Use the infra_* MCP tools. Start with infra_targets if the target is not obvious.

Routing by request type:

Containers / Docker:
- "what's running" → infra_containers()
- "logs for X" → infra_container_logs(name="X", errors_only=true)
- "why is X crashing" → infra_container_logs(name="X", errors_only=true), then infra_container_stats() if OOM suspected
- "compose stacks" → infra_compose()

Systemd services:
- "what services are failing" → infra_services(failed=true)
- "logs for service X" → infra_service_logs(name="X", errors_only=true)

Kubernetes:
- "pods in namespace X" → infra_pods(namespace="X")
- "why is pod X crashing" → infra_pod_logs(pod="X", errors_only=true)
- "deployment status" → infra_deployments()
- "k8s warnings" → infra_k8s_events()

System health:
- "disk usage" → infra_disk()
- "what's using CPU/memory" → infra_processes(sort_by="cpu") or sort_by="mem"

Multi-target: if the user names a VM or cluster, call infra_targets() first to confirm the target name, then pass target= to every subsequent call.

Rules:
- Always use errors_only=true on log tools unless the user explicitly wants full logs
- For "why is X down": check logs first, then stats — explain root cause, don't dump raw logs
- If a kubernetes target is given for a container question, redirect to infra_pods`,
		},
		{
			name:        "diff",
			description: "Show and explain what changed in the working tree or between commits",
			guide: `Use git_diff and git_log to answer this.

Steps:
1. Call git_diff(stat=true) first to get the file-level summary
2. For files the user cares about (or all if few), call git_diff on each file
3. Summarize: what changed, why it likely changed, any risks

For "diff vs <branch>": git_diff(base="<branch>")
For "what changed in commit X": git_show(hash="X")
For "changes since yesterday": git_log to find the boundary commit, then git_diff`,
		},
		{
			name:        "blame",
			description: "Find who wrote specific lines and why — git blame with context",
			guide: `Use git_blame to annotate the file, git_show to get commit context.

Steps:
1. Extract the file path and optional line range from the request
2. Call git_blame(file="<path>", start_line=N, end_line=M)
3. For any interesting commits, call git_show(hash="<hash>") to get the commit message and context
4. Report: author, date, commit message summary, and why those lines exist

If no line range given, blame the whole file but summarize by author rather than listing every line.`,
		},
		{
			name:        "branch",
			description: "List, create, or delete git branches",
			guide: `Use git_branch to manage branches.

- "list branches" → git_branch(action="list") — shows local branches; add remote=true for remote branches
- "create branch X" → git_branch(action="create", name="X")
- "delete branch X" → git_branch(action="delete", name="X")

Note: switching branches is not supported via MCP tools — tell the user to run: git checkout <name>
For "what branch am I on": use git_status instead.`,
		},
	}
}
