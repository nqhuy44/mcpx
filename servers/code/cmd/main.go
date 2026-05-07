package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/servers/code/internal/codeindex"
	"github.com/nqhuy44/mcpx/servers/code/internal/ollama"
)

func main() {
	s := mcpserver.NewMCPServer("mcpx-code", "0.1.0",
		mcpserver.WithToolCapabilities(false),
	)

	registerTools(s, ollama.New())

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "http" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8083"
		}
		srv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-code listening on :%s (HTTP)", port)
		if err := http.ListenAndServe(":"+port, srv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	} else {
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}

func registerTools(s *mcpserver.MCPServer, ol *ollama.Client) {
	// ── code_search ───────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_search",
		mcp.WithDescription("Full-text search across a codebase. Returns file:line matches ranked by file path."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Text or symbol to search for")),
		mcp.WithString("lang", mcp.Description("Filter by language: go, python, ts, js (optional)")),
		mcp.WithNumber("limit", mcp.Description("Max matches to return (default 20)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		matches, err := codeindex.Search(
			req.GetString("repo", ""),
			req.GetString("query", ""),
			req.GetString("lang", ""),
			int(req.GetFloat("limit", 20)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(matches) == 0 {
			return mcp.NewToolResultText("no matches found"), nil
		}
		var sb strings.Builder
		for _, m := range matches {
			fmt.Fprintf(&sb, "%s:%d  %s\n", m.File, m.Line, m.Content)
		}
		return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
	})

	// ── code_find ─────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_find",
		mcp.WithDescription("Locate a symbol (function, type, var, class) by name. Returns definition site, signature, and a preview."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol name or partial name to find")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		syms, err := codeindex.FindSymbol(
			req.GetString("repo", ""),
			req.GetString("symbol", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(syms) == 0 {
			return mcp.NewToolResultText("symbol not found"), nil
		}
		var sb strings.Builder
		for i, s := range syms {
			if i > 0 {
				sb.WriteString("\n---\n")
			}
			fmt.Fprintf(&sb, "symbol  %s\nfile    %s:%d\nsig     %s\npreview %s\n",
				s.Name, s.File, s.Line, s.Signature, s.Preview)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// ── code_callers ──────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_callers",
		mcp.WithDescription("Find all call sites of a function across the codebase. Returns file:line:context for each."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Function name to find callers of")),
		mcp.WithNumber("limit", mcp.Description("Max results to return (default 20)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		matches, err := codeindex.Callers(
			req.GetString("repo", ""),
			req.GetString("symbol", ""),
			int(req.GetFloat("limit", 20)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(matches) == 0 {
			return mcp.NewToolResultText("no callers found"), nil
		}
		var sb strings.Builder
		for _, m := range matches {
			fmt.Fprintf(&sb, "%s:%d  %s\n", m.File, m.Line, m.Content)
		}
		return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
	})

	// ── code_deps ─────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_deps",
		mcp.WithDescription("Show the import/dependency list for a file or package directory. Supports Go and Python."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("path", mcp.Required(), mcp.Description("File or directory path relative to repo root")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		imports, err := codeindex.Deps(
			req.GetString("repo", ""),
			req.GetString("path", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(imports) == 0 {
			return mcp.NewToolResultText("no imports found"), nil
		}
		return mcp.NewToolResultText(strings.Join(imports, "\n")), nil
	})

	// ── code_explain ──────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_explain",
		mcp.WithDescription("Explain a file:line range in plain English using a local LLM (Ollama). Set OLLAMA_URL if not on localhost."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path relative to repo root")),
		mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line to explain (1-based)")),
		mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line to explain (inclusive)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo := req.GetString("repo", "")
		relPath := req.GetString("path", "")
		start := int(req.GetFloat("start_line", 1))
		end := int(req.GetFloat("end_line", 0))

		absPath := filepath.Join(repo, relPath)
		code, err := codeindex.ReadLines(absPath, start, end)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if code == "" {
			return mcp.NewToolResultText("no content at specified range"), nil
		}

		prompt := "Explain what this code does in plain English. Be concise (2-4 sentences). Return only the explanation, no preamble.\n\n" + code
		result, err := ol.Generate(prompt)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	// ── code_diff_review ──────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("code_diff_review",
		mcp.WithDescription("Review a unified diff for correctness, potential bugs, and code smells using a local LLM (Ollama)."),
		mcp.WithString("diff", mcp.Required(), mcp.Description("Unified diff text to review")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diff := req.GetString("diff", "")
		if diff == "" {
			return mcp.NewToolResultError("diff is required"), nil
		}

		prompt := `Review this code diff. Identify correctness issues, potential bugs, and code smells. Format:

ISSUES (if any):
- <description>: <file:line> — <explanation>

SUGGESTIONS (optional):
- <suggestion>

If no issues found, respond: "LGTM — no issues found."

DIFF:
` + diff

		result, err := ol.Generate(prompt)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})
}
