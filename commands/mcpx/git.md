Run a git operation on the current repo.

User request: $ARGUMENTS

Use the git_* MCP tools to complete this. Follow this order:
1. Call git_status first if the request is about current state or "what changed"
2. Call git_log for history/author/timeline questions (use n=10 by default)
3. Call git_diff for content of changes (use stat=true first, then full diff only if needed)
4. Call git_show for a specific commit hash

Keep the response concise — summarize what you found, don't dump raw output.
