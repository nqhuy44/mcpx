package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
	"github.com/nqhuy44/mcpx/proxy/internal/metrics"
	"github.com/nqhuy44/mcpx/proxy/internal/registry"
	"github.com/nqhuy44/mcpx/proxy/internal/transport"
)

type Gateway struct {
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

// Init connects to all domain servers, registering each as connected or error.
// A single server failure does not abort startup — it is recorded in metrics.
func (g *Gateway) Init(ctx context.Context) error {
	for _, sc := range g.cfg.Servers {
		stats := g.collector.RegisterServer(sc.Name)

		client, err := transport.New(sc)
		if err != nil {
			stats.SetStatus("error: " + err.Error())
			continue
		}
		if err := client.Initialize(ctx); err != nil {
			stats.SetStatus("error: " + err.Error())
			continue
		}

		tools, err := client.ListTools(ctx)
		if err != nil {
			stats.SetStatus("error: list tools: " + err.Error())
			continue
		}

		stats.SetStatus("ok")
		g.clients[sc.Name] = client

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
		client, ok := g.clients[serverName]
		if !ok {
			return nil, fmt.Errorf("no client for server %q", serverName)
		}

		argsJSON, _ := json.Marshal(req.GetArguments())
		tokIn := int64(len(argsJSON)) / 4

		result, err := client.CallTool(ctx, toolName, req.GetArguments())

		var tokOut int64
		if result != nil {
			out, _ := json.Marshal(result.Content)
			tokOut = int64(len(out)) / 4
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
