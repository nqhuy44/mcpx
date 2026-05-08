package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/servers/exec/internal/sandbox"
)

func main() {
	s := mcpserver.NewMCPServer("mcpx-exec", "0.1.0",
		mcpserver.WithToolCapabilities(false),
	)

	registerTools(s)

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "http" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8089"
		}
		srv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-exec listening on :%s (HTTP)", port)
		if err := http.ListenAndServe(":"+port, srv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	} else {
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}

func registerTools(s *mcpserver.MCPServer) {
	// ── exec_run ──────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("exec_run",
		mcp.WithDescription("Execute a code snippet and return stdout/stderr/exit code. Runs in an isolated temp directory. Supported: python, javascript/node, bash, go, ruby, php."),
		mcp.WithString("language", mcp.Required(),
			mcp.Description("Runtime: python, javascript, bash, go, ruby, php")),
		mcp.WithString("code", mcp.Required(),
			mcp.Description("Source code to execute")),
		mcp.WithString("stdin",
			mcp.Description("Data to pass to the program's stdin")),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds, 1–60 (default 10)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := sandbox.Run(
			req.GetString("language", ""),
			req.GetString("code", ""),
			req.GetString("stdin", ""),
			int(req.GetFloat("timeout", 10)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatResult(result)), nil
	})

	// ── exec_langs ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("exec_langs",
		mcp.WithDescription("List runtimes available on this machine."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		available := sandbox.AvailableLangs()
		if len(available) == 0 {
			return mcp.NewToolResultText("no supported runtimes found in PATH"), nil
		}
		return mcp.NewToolResultText("available: " + strings.Join(available, ", ")), nil
	})
}

func formatResult(r *sandbox.Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "exit:%d\n", r.ExitCode)
	if r.Stdout != "" {
		sb.WriteString("---\n")
		sb.WriteString(r.Stdout)
	}
	if r.Stderr != "" {
		if r.Stdout != "" {
			sb.WriteByte('\n')
		}
		sb.WriteString("stderr:\n")
		sb.WriteString(r.Stderr)
	}
	if r.Truncated {
		sb.WriteString("\n[output truncated to 8KB per stream]")
	}
	return sb.String()
}
