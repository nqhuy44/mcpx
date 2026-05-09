Execute a shell command or code snippet and return filtered output.

User request: $ARGUMENTS

Use exec_run for two cases:

**1. Project commands (test/build)** — pass cmd + workdir + filter:
- "run tests" → call_tool(name="exec_run", args={"cmd":"go test ./...","workdir":"<project_path>","filter":"test"})
- "build" → call_tool(name="exec_run", args={"cmd":"make build","workdir":"<project_path>","filter":"build"})
- filter=test returns only failing tests + summary (drops passing output)
- filter=build returns only error lines (drops warnings)

**2. Code snippets** — pass code + lang (isolated temp dir):
- call_tool(name="exec_run", args={"code":"...","lang":"python"})

Rules:
- Always pass workdir for project commands
- Always use filter=test or filter=build — never dump raw test/build output
- Default timeout 30s; raise to 60s for slow suites
- On failure: read the error lines, explain root cause, suggest a fix
