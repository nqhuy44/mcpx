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
	// Two modes:
	//   cmd mode:  run a shell command in workdir (for project test/build)
	//   code mode: run a language snippet in a temp dir (for quick scripts)
	//
	// filter=test strips passing tests — only failures + summary are returned.
	// filter=build strips warnings — only error lines are returned.
	s.AddTool(mcp.NewTool("exec_run",
		mcp.WithDescription("Run a shell command or code snippet. filter=test|build extracts only failures from verbose output."),
		mcp.WithString("cmd", mcp.Description("Shell command e.g. 'go test ./...' or 'make build' (runs in workdir)")),
		mcp.WithString("workdir", mcp.Description("Directory to run cmd in (required for project commands)")),
		mcp.WithString("code", mcp.Description("Code snippet to run (alternative to cmd, runs in temp dir)")),
		mcp.WithString("lang", mcp.Description("Snippet language: python|javascript|bash|go|ruby|php")),
		mcp.WithString("filter", mcp.Description("Output filter: none (default)|test|build")),
		mcp.WithNumber("timeout", mcp.Description("Timeout seconds 1-60 (default: 30 for cmd, 10 for snippet)")),
		mcp.WithString("stdin", mcp.Description("Data for stdin (snippet mode only)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filter := req.GetString("filter", "none")
		timeout := int(req.GetFloat("timeout", 0))

		var result *sandbox.Result
		var err error

		if cmd := req.GetString("cmd", ""); cmd != "" {
			result, err = sandbox.RunCmd(cmd, req.GetString("workdir", ""), timeout)
		} else if code := req.GetString("code", ""); code != "" {
			result, err = sandbox.Run(
				req.GetString("lang", "bash"),
				code,
				req.GetString("stdin", ""),
				timeout,
			)
		} else {
			return mcp.NewToolResultError("provide either cmd or code"), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var out string
		switch filter {
		case "test":
			out = sandbox.FilterTest(result)
		case "build":
			out = sandbox.FilterBuild(result)
		default:
			out = formatRaw(result)
		}

		return mcp.NewToolResultText(out), nil
	})

	// ── exec_langs ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("exec_langs",
		mcp.WithDescription("List runtimes available on this machine for snippet execution."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		available := sandbox.AvailableLangs()
		if len(available) == 0 {
			return mcp.NewToolResultText("no supported runtimes found in PATH"), nil
		}
		return mcp.NewToolResultText("available: " + strings.Join(available, ", ")), nil
	})
}

func formatRaw(r *sandbox.Result) string {
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
