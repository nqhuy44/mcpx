Show and explain the current diff or changes between refs.

User request: $ARGUMENTS

Use git_diff. Strategy:
1. Call git_diff with stat=true first to get the file-level summary
2. If the user wants details on specific files, call git_diff again with path=<file> and stat=false
3. For branch comparison use ref="main" (or the base branch name)

Summarize what changed and why it matters — don't just list files.
Flag anything suspicious: large deletions, config changes, binary files.
