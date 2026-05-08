Search, navigate, or explain code in the current codebase.

User request: $ARGUMENTS

Use the code_* MCP tools. Pick the right tool for the request:

- "find function/type/const X" → code_find(symbol="X")
- "search for 'auth token'" → code_search(query="auth token") — use path= to narrow to a dir
- "who calls X" / "where is X used" → code_callers(symbol="X")
- "what does file X import" / "dependencies of X" → code_deps(path="X")
- "explain lines N–M of file X" → code_explain(file="X", start_line=N, end_line=M)
- "review this diff" / "what's wrong with this change" → code_diff_review(diff="<diff text>")

Chaining:
- For "find and explain X": code_find first to get the file:line, then code_explain on that range
- For "trace how X reaches Y": code_callers(X) to find callers, repeat up the chain (max 3 hops)
- For "what would break if I change X": code_callers(X) + code_deps to map blast radius

Rules:
- code_explain and code_diff_review require Ollama — if they error, tell the user to run: ollama run qwen2.5-coder:7b
- Keep explanations concise — summarize what the code does and why, don't re-read every line back
- For large search results (>20 hits), group by file and summarize rather than listing all matches
