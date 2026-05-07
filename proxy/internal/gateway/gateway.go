package gateway

import (
	"context"
	"encoding/json"
	"fmt"
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
// Tool calls to that server return a descriptive error until re-enabled.
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
// It re-uses the server config from gateway.yaml.
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
	)

	searchTool := mcp.NewTool("search_tools",
		mcp.WithDescription("Search available tools by keyword. Returns top-3 matches with name, description, and input schema."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keywords to search for")),
	)
	s.AddTool(searchTool, g.handleSearchTools)

	for _, entry := range g.reg.All() {
		g.registerPassthrough(s, entry)
	}

	g.mcpSrv = s
	return s
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
		fmt.Fprintf(&sb, "[%d] %s | server:%s\n  desc: %s\n  schema: %s\n",
			i+1, e.Name, e.ServerName, e.Description, string(e.InputSchema))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func (g *Gateway) registerPassthrough(s *mcpserver.MCPServer, entry *registry.ToolEntry) {
	t := mcp.NewTool(entry.Name, mcp.WithDescription(entry.Description))

	serverName := entry.ServerName
	toolName := entry.Name
	stats := g.collector.Server(serverName)

	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if stats != nil && stats.GetStatus() == "disabled" {
			return mcp.NewToolResultError(
				"server '" + serverName + "' is disabled — enable it from the admin dashboard (/ui)",
			), nil
		}

		g.mu.RLock()
		client, ok := g.clients[serverName]
		g.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("no client for server %q", serverName)
		}

		argsJSON, _ := json.Marshal(req.GetArguments())
		tokIn := metrics.EstimateTokens(argsJSON)

		result, err := client.CallTool(ctx, toolName, req.GetArguments())

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
	})
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
