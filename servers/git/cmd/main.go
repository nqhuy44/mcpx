package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/servers/git/internal/gitops"
	"github.com/nqhuy44/mcpx/servers/git/internal/github"
)

func main() {
	s := mcpserver.NewMCPServer("mcpx-git", "0.1.0",
		mcpserver.WithToolCapabilities(false),
	)

	ghToken := os.Getenv("GITHUB_TOKEN")
	ghClient := github.New(ghToken)

	registerTools(s, ghClient)

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "http" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8081"
		}
		srv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-git listening on :%s (HTTP)", port)
		if err := http.ListenAndServe(":"+port, srv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	} else {
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}

func registerTools(s *mcpserver.MCPServer, gh *github.Client) {
	// ── git_status ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_status",
		mcp.WithDescription("Show working tree status (staged, unstaged, untracked files) for a repo."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo := req.GetString("repo", "")
		out, err := gitops.Status(repo)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── git_log ───────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_log",
		mcp.WithDescription("Show recent commit history. Returns hash, author, date, subject. Filterable by author, path, or date range."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
		mcp.WithNumber("n", mcp.Description("Number of commits to return (default 20)")),
		mcp.WithString("author", mcp.Description("Filter by author name/email substring")),
		mcp.WithString("path", mcp.Description("Filter commits touching this path")),
		mcp.WithString("since", mcp.Description("Show commits more recent than a date (e.g. '2 weeks ago', '2024-01-01')")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo := req.GetString("repo", "")
		n := int(req.GetFloat("n", 20))
		entries, err := gitops.Log(repo,
			req.GetString("author", ""),
			req.GetString("path", ""),
			req.GetString("since", ""),
			n,
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(entries) == 0 {
			return mcp.NewToolResultText("no commits found"), nil
		}
		var sb strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&sb, "%s  %s  %s  %s\n", e.Hash, e.Date, e.Author, e.Subject)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// ── git_diff ──────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_diff",
		mcp.WithDescription("Show diff for a file, commit range, or staged changes. Use ref='staged' for the index, a commit hash or 'HEAD~1..HEAD' for a range."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
		mcp.WithString("ref", mcp.Description("'staged', a commit hash, or a range like HEAD~1..HEAD (default: working tree)")),
		mcp.WithString("path", mcp.Description("Limit diff to this file or directory")),
		mcp.WithBoolean("stat", mcp.Description("Return only a summary (--stat) instead of full diff")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := gitops.Diff(
			req.GetString("repo", ""),
			req.GetString("ref", ""),
			req.GetString("path", ""),
			req.GetBool("stat", false),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── git_branch ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_branch",
		mcp.WithDescription("List, create, or delete local branches."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
		mcp.WithString("action", mcp.Description("'list' (default), 'create', or 'delete'")),
		mcp.WithString("name", mcp.Description("Branch name (required for create/delete)")),
		mcp.WithString("base", mcp.Description("Base ref for 'create' (default: HEAD)")),
		mcp.WithBoolean("force", mcp.Description("Force-delete branch even if unmerged")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repo := req.GetString("repo", "")
		action := req.GetString("action", "list")
		switch action {
		case "create":
			name := req.GetString("name", "")
			if name == "" {
				return mcp.NewToolResultError("name is required for create"), nil
			}
			out, err := gitops.BranchCreate(repo, name, req.GetString("base", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "branch created: " + name
			}
			return mcp.NewToolResultText(out), nil
		case "delete":
			name := req.GetString("name", "")
			if name == "" {
				return mcp.NewToolResultError("name is required for delete"), nil
			}
			out, err := gitops.BranchDelete(repo, name, req.GetBool("force", false))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "branch deleted: " + name
			}
			return mcp.NewToolResultText(out), nil
		default:
			out, err := gitops.BranchList(repo)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(out), nil
		}
	})

	// ── git_show ──────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_show",
		mcp.WithDescription("Show details of a single commit: message, author, date, and changed-file summary."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
		mcp.WithString("ref", mcp.Description("Commit hash or ref (default: HEAD)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := gitops.Show(
			req.GetString("repo", ""),
			req.GetString("ref", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── git_blame ─────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("git_blame",
		mcp.WithDescription("Annotate lines of a file with the commit and author that last changed each line."),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Absolute path to the git repository")),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path relative to repo root")),
		mcp.WithNumber("start_line", mcp.Description("First line of range (1-based)")),
		mcp.WithNumber("end_line", mcp.Description("Last line of range (inclusive)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := gitops.Blame(
			req.GetString("repo", ""),
			req.GetString("path", ""),
			int(req.GetFloat("start_line", 0)),
			int(req.GetFloat("end_line", 0)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── github_pr_list ────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("github_pr_list",
		mcp.WithDescription("List pull requests for a GitHub repo. Returns number, title, author, state, branch, labels, date."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("GitHub org or user")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name")),
		mcp.WithString("state", mcp.Description("'open' (default), 'closed', or 'all'")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prs, err := gh.PRList(
			req.GetString("owner", ""),
			req.GetString("repo", ""),
			req.GetString("state", "open"),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(prs) == 0 {
			return mcp.NewToolResultText("no PRs found"), nil
		}
		var sb strings.Builder
		for _, p := range prs {
			draft := ""
			if p.Draft {
				draft = " [draft]"
			}
			labels := ""
			if len(p.Labels) > 0 {
				labels = " labels:" + strings.Join(p.Labels, ",")
			}
			fmt.Fprintf(&sb, "#%d %s%s  %s→%s  by:%s  %s%s\n",
				p.Number, p.Title, draft, p.Head, p.Base, p.Author, p.CreatedAt, labels)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// ── github_pr_get ─────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("github_pr_get",
		mcp.WithDescription("Get details of a single GitHub PR: description (truncated), diff stats, mergeable status."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("GitHub org or user")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name")),
		mcp.WithNumber("number", mcp.Required(), mcp.Description("PR number")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pr, err := gh.PRGet(
			req.GetString("owner", ""),
			req.GetString("repo", ""),
			int(req.GetFloat("number", 0)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		mergeable := "unknown"
		if pr.Mergeable != nil {
			if *pr.Mergeable {
				mergeable = "yes"
			} else {
				mergeable = "no"
			}
		}
		title := pr.Title
		if pr.Draft {
			title += " [draft]"
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "#%d %s [%s]\n", pr.Number, title, pr.State)
		fmt.Fprintf(&sb, "author:%s  head:%s→%s  mergeable:%s  created:%s\n",
			pr.Author, pr.Head, pr.Base, mergeable, pr.CreatedAt)
		fmt.Fprintf(&sb, "+%d -%d  files:%d", pr.Additions, pr.Deletions, pr.ChangedFiles)
		if len(pr.Labels) > 0 {
			fmt.Fprintf(&sb, "  labels:%s", strings.Join(pr.Labels, ","))
		}
		fmt.Fprintf(&sb, "\nurl:%s\n", pr.URL)
		if pr.Body != "" {
			fmt.Fprintf(&sb, "body: %s\n", pr.Body)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// ── github_pr_comment ─────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("github_pr_comment",
		mcp.WithDescription("Post a comment on a GitHub PR or issue."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("GitHub org or user")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name")),
		mcp.WithNumber("number", mcp.Required(), mcp.Description("PR or issue number")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Comment text (Markdown supported)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		err := gh.PRComment(
			req.GetString("owner", ""),
			req.GetString("repo", ""),
			int(req.GetFloat("number", 0)),
			req.GetString("body", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("comment posted on #" + strconv.Itoa(int(req.GetFloat("number", 0)))), nil
	})
}
