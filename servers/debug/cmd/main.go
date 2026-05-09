package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/servers/debug/internal/debug"
)

func main() {
	s := mcpserver.NewMCPServer("mcpx-debug", "0.1.0",
		mcpserver.WithToolCapabilities(false),
	)

	s.AddTool(mcp.NewTool("debug_analyze",
		mcp.WithDescription("Analyze a stacktrace, panic, log blob, or stderr — extracts errors, filters noise, returns actionable summary. Accepts Go/Python/Node/Java/Rust."),
		mcp.WithString("input", mcp.Description("Raw text: stacktrace, panic output, log lines, or stderr")),
		mcp.WithString("file", mcp.Description("Path to a log or crash file (alternative to input)")),
		mcp.WithString("type", mcp.Description("auto (default)|stacktrace|panic|logs|stderr")),
		mcp.WithString("lang", mcp.Description("auto (default)|go|python|node|java|rust")),
		mcp.WithNumber("context", mcp.Description("Lines of context around each error in log mode (default: 3)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input := req.GetString("input", "")

		if filePath := req.GetString("file", ""); filePath != "" && input == "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return mcp.NewToolResultError("cannot read file: " + err.Error()), nil
			}
			input = string(data)
		}

		if input == "" {
			return mcp.NewToolResultError("provide input text or file path"), nil
		}

		out := debug.Analyze(
			input,
			req.GetString("type", "auto"),
			req.GetString("lang", "auto"),
			int(req.GetFloat("context", 3)),
		)
		return mcp.NewToolResultText(out), nil
	})

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "http" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8087"
		}
		srv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-debug listening on :%s (HTTP)", port)
		if err := http.ListenAndServe(":"+port, srv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	} else {
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}
