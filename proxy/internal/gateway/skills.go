package gateway

// skill defines one invokable MCP Prompt — a usage guide for a specific server.
// Each skill is registered as a prompt named "mcpx_<name>" with an optional
// "request" argument. Any MCP client that supports prompts/list can invoke them
// as slash commands (e.g. /mcpx_exec, /mcpx_infra).
type skill struct {
	name        string
	description string // shown in client prompt pickers
	guide       string // instruction body; "request" argument is appended at runtime
}

func skills() []skill {
	return []skill{
		{
			name:        "exec",
			description: "Run a test suite, build, or code snippet and return filtered output",
			guide: `Use exec_run via call_tool for two cases:

1. Project commands (test/build) — pass cmd + workdir + filter:
   - "run tests" → call_tool(name="exec_run", args={"cmd":"go test ./...","workdir":"<project_path>","filter":"test"})
   - "build" → call_tool(name="exec_run", args={"cmd":"make build","workdir":"<project_path>","filter":"build"})
   - "run npm test" → call_tool(name="exec_run", args={"cmd":"npm test","workdir":"<project_path>","filter":"test"})
   filter=test: returns only failing tests + summary, drops all passing output
   filter=build: returns only error lines, drops warnings

2. Code snippets — pass code + lang (isolated temp dir):
   - call_tool(name="exec_run", args={"code":"print('hello')","lang":"python"})

Rules:
- Always pass workdir for project commands — exec_run won't find project files without it
- Always use filter=test for test runs and filter=build for builds
- Default timeout 30s; raise to 60s for slow suites
- On failure: read the error lines, explain root cause, suggest a fix`,
		},
		{
			name:        "debug",
			description: "Analyze a stacktrace, panic output, log file, or stderr blob — extracts errors, filters noise, returns actionable summary",
			guide: `Use debug_analyze via call_tool to triage errors without pasting raw noise into context.

Inputs:
- Paste raw text:  call_tool(name="debug_analyze", args={"input":"<raw text>"})
- Read a file:     call_tool(name="debug_analyze", args={"file":"/path/to/app.log"})

Type is auto-detected (stacktrace|panic|logs|stderr). Lang is auto-detected (go|python|node|java|rust).

Override when needed:
- call_tool(name="debug_analyze", args={"input":"...","type":"logs","context":5})
- call_tool(name="debug_analyze", args={"input":"...","lang":"python"})

Output: error message, relevant frames (stdlib skipped), CAUSE suggestion if pattern matched.
For logs: shows only ERROR/FATAL lines + N lines of context, drops INFO/DEBUG noise.

Rules:
- Always use debug_analyze instead of pasting raw stacktraces or log blobs directly
- For "why is X failing": analyze the stacktrace/log first, then explain root cause`,
		},
	}
}
