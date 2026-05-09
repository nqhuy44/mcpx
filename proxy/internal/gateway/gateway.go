package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
	"github.com/nqhuy44/mcpx/proxy/internal/metrics"
	"github.com/nqhuy44/mcpx/proxy/internal/registry"
	"github.com/nqhuy44/mcpx/proxy/internal/transport"
)

type Gateway struct {
	mu        sync.RWMutex
	cfg       *config.Config
	reg       *registry.Registry
	clients   map[string]transport.Client
	collector *metrics.Collector
	mcpSrv    *mcpserver.MCPServer
}

func New(cfg *config.Config, collector *metrics.Collector) *Gateway {
	return &Gateway{
		cfg:       cfg,
		reg:       registry.New(),
		clients:   make(map[string]transport.Client),
		collector: collector,
	}
}

// Init connects to all enabled domain servers. A single server failure does
// not abort startup — it is recorded in metrics. Servers marked disabled:true
// in config are registered with status "disabled" but not connected.
func (g *Gateway) Init(ctx context.Context) error {
	for _, sc := range g.cfg.Servers {
		stats := g.collector.RegisterServer(sc.Name)

		if sc.Disabled {
			stats.SetStatus("disabled")
			continue
		}

		g.initServer(ctx, sc, stats)
	}
	return nil
}

// initServer connects one server and registers its tools. Called from Init and
// EnableServer. Caller must NOT hold g.mu.
func (g *Gateway) initServer(ctx context.Context, sc config.ServerConfig, stats *metrics.ServerStats) {
	client, err := transport.New(sc)
	if err != nil {
		stats.SetStatus("error: " + err.Error())
		return
	}
	if err := client.Initialize(ctx); err != nil {
		stats.SetStatus("error: " + err.Error())
		return
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		stats.SetStatus("error: list tools: " + err.Error())
		return
	}

	stats.SetStatus("ok")

	g.mu.Lock()
	g.clients[sc.Name] = client
	g.mu.Unlock()

	for _, t := range tools {
		schema, _ := json.Marshal(t.InputSchema)
		g.reg.Register(&registry.ToolEntry{
			Name:        t.Name,
			ServerName:  sc.Name,
			Description: t.Description,
			InputSchema: schema,
			Keywords:    keywordsFrom(t.Name, t.Description),
		})
	}
}

// DisableServer stops the named server and marks it disabled.
func (g *Gateway) DisableServer(name string) {
	g.mu.Lock()
	client, had := g.clients[name]
	delete(g.clients, name)
	g.mu.Unlock()

	if had {
		client.Close() //nolint:errcheck
	}

	if stats := g.collector.Server(name); stats != nil {
		stats.SetStatus("disabled")
	}
}

// EnableServer (re-)connects a previously disabled or failed server.
func (g *Gateway) EnableServer(ctx context.Context, name string) error {
	var sc config.ServerConfig
	var found bool
	for _, s := range g.cfg.Servers {
		if s.Name == name {
			sc = s
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server %q not found in config", name)
	}

	stats := g.collector.Server(name)
	if stats == nil {
		stats = g.collector.RegisterServer(name)
	}
	stats.SetStatus("connecting")

	client, err := transport.New(sc)
	if err != nil {
		stats.SetStatus("error: " + err.Error())
		return err
	}
	if err := client.Initialize(ctx); err != nil {
		stats.SetStatus("error: " + err.Error())
		return err
	}
	if _, err := client.ListTools(ctx); err != nil {
		stats.SetStatus("error: list tools: " + err.Error())
		return err
	}

	stats.SetStatus("ok")
	g.mu.Lock()
	g.clients[name] = client
	g.mu.Unlock()

	return nil
}

func (g *Gateway) BuildMCPServer() *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("mcpx-proxy", "0.1.0",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithPromptCapabilities(false),
	)

	// ── call_tool: universal dispatcher ──────────────────────────────────────
	// Only tool the LLM needs to invoke any backend tool. Schemas are NOT loaded
	// into the client context — the LLM learns tool names and args from the
	// mcpx_usage prompt (injected at session start) or mcpx_guide.

	s.AddTool(mcp.NewTool("call_tool",
		mcp.WithDescription("Invoke any mcpx tool by name. Use mcpx_guide or search_tools to find tool names and args."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Tool name e.g. git, code, infra_vm")),
		mcp.WithString("args", mcp.Description(`JSON args e.g. {"action":"log","n":5,"repo":"/path"}`)),
	), g.handleCallTool)

	// ── search_tools: discovery ───────────────────────────────────────────────

	s.AddTool(mcp.NewTool("search_tools",
		mcp.WithDescription("Find a tool by keyword. Returns name, description, and schema. Then call it via call_tool."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keywords to search for")),
	), g.handleSearchTools)

	// ── mcpx_guide: routing reference ────────────────────────────────────────

	s.AddTool(mcp.NewTool("mcpx_guide",
		mcp.WithDescription("Full routing guide: all tool names and their args. Call once at session start."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(usageGuide(g.reg.All())), nil
	})

	// ── mcpx_usage prompt: auto-injected at session start ─────────────────────

	s.AddPrompt(mcp.NewPrompt("mcpx_usage",
		mcp.WithPromptDescription("System context for mcpx: call_tool dispatch pattern and routing table. Injected at session start by clients that support MCP prompts."),
	), func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult(
			"mcpx tool usage guide",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(
					mcp.RoleUser,
					mcp.NewTextContent(usageGuide(g.reg.All())),
				),
			},
		), nil
	})

	// per-skill prompts: slash commands for any client that supports prompts/list

	for _, sk := range skills() {
		sk := sk
		s.AddPrompt(mcp.NewPrompt("mcpx_"+sk.name,
			mcp.WithPromptDescription(sk.description),
			mcp.WithArgument("request", mcp.ArgumentDescription("What to do")),
		), func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			body := sk.guide
			if r := req.Params.Arguments["request"]; r != "" {
				body += "\n\nUser request: " + r
			}
			return mcp.NewGetPromptResult(sk.description, []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(body)),
			}), nil
		})
	}

	g.mcpSrv = s
	return s
}

// handleCallTool dispatches a call_tool(name, args) request to the correct
// backend server. args is a JSON object string; it is parsed and forwarded
// as the tool's argument map.
func (g *Gateway) handleCallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	argsStr := req.GetString("args", "")
	var args map[string]interface{}
	if argsStr != "" {
		if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
			return mcp.NewToolResultError("args must be valid JSON: " + err.Error()), nil
		}
	}

	entry, ok := g.reg.Get(name)
	if !ok {
		matches := g.reg.Search(name, 3)
		if len(matches) > 0 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return mcp.NewToolResultError("unknown tool: " + name + " — did you mean: " + strings.Join(names, ", ") + "? Call search_tools for more."), nil
		}
		return mcp.NewToolResultError("unknown tool: " + name + " — call search_tools(query) to discover available tools"), nil
	}

	stats := g.collector.Server(entry.ServerName)
	if stats != nil && stats.GetStatus() == "disabled" {
		return mcp.NewToolResultError("server '" + entry.ServerName + "' is disabled — enable it from the admin dashboard (/ui)"), nil
	}

	g.mu.RLock()
	client, ok := g.clients[entry.ServerName]
	g.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no client for server %q", entry.ServerName)
	}

	argsJSON, _ := json.Marshal(args)
	tokIn := metrics.EstimateTokens(argsJSON)

	result, err := client.CallTool(ctx, name, args)

	var tokOut int64
	if result != nil {
		out, _ := json.Marshal(result.Content)
		tokOut = metrics.EstimateTokens(out)
	}

	if stats != nil {
		isError := err != nil || (result != nil && result.IsError)
		stats.RecordCall(tokIn, tokOut, isError)
	}

	return result, err
}

// usageGuide builds a compact routing table. Shown via mcpx_guide and
// injected as the mcpx_usage prompt — this replaces per-tool schema loading.
func usageGuide(entries []*registry.ToolEntry) string {
	// group and sort tools by server, then by name within each server
	byServer := make(map[string][]*registry.ToolEntry)
	var order []string
	for _, e := range entries {
		if _, seen := byServer[e.ServerName]; !seen {
			order = append(order, e.ServerName)
		}
		byServer[e.ServerName] = append(byServer[e.ServerName], e)
	}
	sort.Strings(order)
	for _, tools := range byServer {
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	}

	var sb strings.Builder
	sb.WriteString("# mcpx tool guide\n\n")
	sb.WriteString("Invoke tools via: call_tool(name=\"<tool>\", args={...})\n")
	sb.WriteString("Discover tools via: search_tools(query) — returns name, description, schema\n\n")

	for _, srv := range order {
		fmt.Fprintf(&sb, "## %s\n", srv)
		for _, e := range byServer[srv] {
			fmt.Fprintf(&sb, "- %s: %s\n", e.Name, e.Description)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("Rule: never ask the user to run a command if a tool can do it.\n")

	return sb.String()
}

func (g *Gateway) handleSearchTools(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultText("no query provided"), nil
	}

	matches := g.reg.Search(query, 3)
	if len(matches) == 0 {
		return mcp.NewToolResultText("no tools matched: " + query), nil
	}

	var sb strings.Builder
	for i, e := range matches {
		fmt.Fprintf(&sb, "[%d] %s | server:%s\n  desc: %s\n  schema: %s\n  invoke: call_tool(name=%q, args={...})\n",
			i+1, e.Name, e.ServerName, e.Description, string(e.InputSchema), e.Name)
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func keywordsFrom(name, desc string) []string {
	raw := strings.ToLower(name + " " + desc)
	raw = strings.NewReplacer("_", " ", "-", " ").Replace(raw)
	tokens := strings.Fields(raw)
	seen := make(map[string]struct{}, len(tokens))
	var out []string
	for _, tok := range tokens {
		if _, dup := seen[tok]; !dup {
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}
	return out
}
