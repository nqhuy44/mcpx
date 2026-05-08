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
	"github.com/nqhuy44/mcpx/servers/infra/internal/infra"
	"github.com/nqhuy44/mcpx/servers/infra/internal/runner"
)

func main() {
	pool, err := runner.NewPool()
	if err != nil {
		log.Fatalf("infra pool: %v", err)
	}
	defer pool.Close()

	s := mcpserver.NewMCPServer("mcpx-infra", "0.2.0",
		mcpserver.WithToolCapabilities(false),
	)

	registerTools(s, pool)

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "http" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8088"
		}
		srv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-infra listening on :%s (HTTP)", port)
		if err := http.ListenAndServe(":"+port, srv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	} else {
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}

func registerTools(s *mcpserver.MCPServer, pool *runner.Pool) {
	// ── infra_targets ─────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_targets",
		mcp.WithDescription("List all auto-detected execution targets: local machine, SSH hosts from ~/.ssh/config, and Kubernetes contexts from ~/.kube/config."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		targets := pool.All()
		var sb strings.Builder
		sb.WriteString("NAME\tTYPE\tDEFAULT\n")
		for _, t := range targets {
			def := "-"
			if t.Default {
				def = "yes"
			}
			fmt.Fprintf(&sb, "%s\t%s\t%s\n", t.Name, t.Type, def)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	// ── infra_containers ──────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_containers",
		mcp.WithDescription("List Docker containers with status, image, and ports. Defaults to running containers only."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
		mcp.WithBoolean("all", mcp.Description("Include stopped containers (default: running only)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Containers(ctx, run, req.GetBool("all", false))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_container_logs ──────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_container_logs",
		mcp.WithDescription("Fetch logs for a Docker container. Use errors_only=true to filter to error/warning lines."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Container name or ID")),
		mcp.WithNumber("lines", mcp.Description("Number of log lines to fetch (default 50)")),
		mcp.WithBoolean("errors_only", mcp.Description("Return only error/warning lines (default false)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.ContainerLogs(ctx, run,
			req.GetString("name", ""),
			int(req.GetFloat("lines", 50)),
			req.GetBool("errors_only", false),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_container_stats ─────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_container_stats",
		mcp.WithDescription("One-shot CPU and memory snapshot for all running Docker containers."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.ContainerStats(ctx, run)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_compose ─────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_compose",
		mcp.WithDescription("List docker-compose stacks and their running service status."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Compose(ctx, run)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_services ────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_services",
		mcp.WithDescription("List systemd services with load/active/sub state. Set failed=true to show only failed services."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
		mcp.WithBoolean("failed", mcp.Description("Show only failed services (default: all services)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Services(ctx, run, req.GetBool("failed", false))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_service_logs ────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_service_logs",
		mcp.WithDescription("Get journald logs for a systemd service. Use errors_only=true to filter to error/warning lines."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Service name, e.g. nginx or api")),
		mcp.WithNumber("lines", mcp.Description("Number of log lines (default 50)")),
		mcp.WithBoolean("errors_only", mcp.Description("Return only error/warning lines (default false)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.ServiceLogs(ctx, run,
			req.GetString("name", ""),
			int(req.GetFloat("lines", 50)),
			req.GetBool("errors_only", false),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_disk ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_disk",
		mcp.WithDescription("Disk usage summary — real mount points sorted by usage percentage."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Disk(ctx, run)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_processes ───────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_processes",
		mcp.WithDescription("Top processes by CPU or memory usage."),
		mcp.WithString("target", mcp.Description("Target name from infra_targets (default: local)")),
		mcp.WithString("sort_by", mcp.Description("Sort by: cpu (default) or mem")),
		mcp.WithNumber("limit", mcp.Description("Number of processes to show (default 10, max 50)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		run, err := pool.VMRunner(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.TopProcesses(ctx, run,
			req.GetString("sort_by", "cpu"),
			int(req.GetFloat("limit", 10)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_pods ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_pods",
		mcp.WithDescription("List Kubernetes pods with status, restarts, age, and node."),
		mcp.WithString("target", mcp.Description("Kubernetes target from infra_targets (default: current-context)")),
		mcp.WithString("namespace", mcp.Description("Namespace to filter (default: all namespaces)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kubeCtx, err := pool.KubeContext(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Pods(ctx, kubeCtx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_pod_logs ────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_pod_logs",
		mcp.WithDescription("Fetch logs for a Kubernetes pod. Use errors_only=true to filter to error/warning lines."),
		mcp.WithString("target", mcp.Description("Kubernetes target from infra_targets (default: current-context)")),
		mcp.WithString("namespace", mcp.Description("Namespace of the pod")),
		mcp.WithString("pod", mcp.Required(), mcp.Description("Pod name")),
		mcp.WithNumber("lines", mcp.Description("Number of log lines to fetch (default 50)")),
		mcp.WithBoolean("errors_only", mcp.Description("Return only error/warning lines (default false)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kubeCtx, err := pool.KubeContext(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.PodLogs(ctx, kubeCtx,
			req.GetString("namespace", ""),
			req.GetString("pod", ""),
			int(req.GetFloat("lines", 50)),
			req.GetBool("errors_only", false),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_deployments ─────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_deployments",
		mcp.WithDescription("List Kubernetes deployments with replica status."),
		mcp.WithString("target", mcp.Description("Kubernetes target from infra_targets (default: current-context)")),
		mcp.WithString("namespace", mcp.Description("Namespace to filter (default: all namespaces)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kubeCtx, err := pool.KubeContext(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.Deployments(ctx, kubeCtx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})

	// ── infra_k8s_events ──────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("infra_k8s_events",
		mcp.WithDescription("List Kubernetes warning events sorted by timestamp."),
		mcp.WithString("target", mcp.Description("Kubernetes target from infra_targets (default: current-context)")),
		mcp.WithString("namespace", mcp.Description("Namespace to filter (default: all namespaces)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kubeCtx, err := pool.KubeContext(req.GetString("target", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := infra.K8sEvents(ctx, kubeCtx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	})
}
