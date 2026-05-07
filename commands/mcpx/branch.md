Manage git branches.

User request: $ARGUMENTS

Use git_branch with the appropriate action:

- "list branches" / "what branches exist" → action=list
- "create branch <name>" → action=create, branch=<name>
- "delete branch <name>" → action=delete, branch=<name>
- "switch to <name>" → note: switching is not supported via MCP; tell the user to run `git checkout <name>` locally

For list: highlight the current branch and flag stale branches (no recent commits).
For create/delete: confirm the action completed and what the next step is.
