Execute a code snippet or shell command and return the output.

User request: $ARGUMENTS

Use exec_run — not the native Bash tool — for:
- Code snippets in any language (python, javascript, bash, go, ruby, php)
- Shell commands with potentially large output: find, grep, cat large files, ps, df, docker ps

Rules:
- Call exec_langs first if the language is unclear
- Default timeout 10s; raise to 30s for I/O-heavy or package-installing code
- Pass stdin if the code reads from stdin
- On non-zero exit: show stderr, explain the error, suggest a fix
- Summarize large outputs — don't dump raw lines (exec_run caps at 8KB anyway)
- Exception: use Bash for commands that need the project directory (make, git, go build) — exec_run runs in an isolated temp dir
