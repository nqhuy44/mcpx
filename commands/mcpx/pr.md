Manage or review a GitHub pull request.

User request: $ARGUMENTS

Use the github_pr_* MCP tools. Common workflows:

- "list open PRs" → github_pr_list(state=open)
- "review PR 42" → github_pr_get(number=42), summarize title/body/changed files/additions/deletions
- "comment on PR 42: <message>" → github_pr_comment(number=42, body=<message>)
- "show my PRs" → github_pr_list, filter by author name from the results

For reviews: highlight key changes, flag large diffs (>500 lines), note if draft.
Always infer owner/repo from the current git remote — run git_status if unsure.
