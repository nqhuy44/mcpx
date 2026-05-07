Find out who wrote a piece of code and when.

User request: $ARGUMENTS

Use git_blame. Extract from the request:
- file path (required)
- line range if mentioned (e.g. "lines 10-30" → start_line=10, end_line=30)

After getting blame output, answer the question directly:
- "who wrote this?" → name the author and date
- "why was this added?" → note the commit hash, then use git_show to get the commit message
- For large ranges, group consecutive lines by the same author/commit
