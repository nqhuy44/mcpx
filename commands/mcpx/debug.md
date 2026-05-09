Use debug_analyze via call_tool to triage errors without pasting raw noise into context.

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
- For "why is X failing": analyze the stacktrace/log first, then explain root cause

$ARGUMENTS
