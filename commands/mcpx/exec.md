Execute a code snippet and return the output.

User request: $ARGUMENTS

Use exec_run. Rules:
- Call exec_langs first if the user hasn't specified a language and it's unclear what's available
- Default timeout is 10s; raise to 30s for anything that does I/O or installs packages
- Pass stdin if the user's code reads from stdin
- On non-zero exit: show stderr, explain the error, suggest a fix
- Truncate display of large outputs — summarize instead of dumping raw lines
