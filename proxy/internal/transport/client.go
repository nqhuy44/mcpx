package transport

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
)

// Client wraps an mcp-go client and exposes the subset of operations the
// gateway needs. The underlying mcp-go client already handles JSON-RPC 2.0
// framing, request IDs, and async response correlation.
type Client interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]mcp.Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error)
	Close() error
}

// New constructs the correct Client implementation based on the server config.
func New(cfg config.ServerConfig) (Client, error) {
	switch cfg.Transport {
	case "stdio":
		return newStdioClient(cfg)
	case "http":
		return newHTTPClient(cfg)
	default:
		return nil, fmt.Errorf("unknown transport %q for server %q", cfg.Transport, cfg.Name)
	}
}

// baseClient embeds an mcp-go *client.Client and satisfies Client.
type baseClient struct {
	inner *mcpclient.Client
}

func (b *baseClient) Initialize(ctx context.Context) error {
	_, err := b.inner.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "mcpx-proxy",
				Version: "0.1.0",
			},
		},
	})
	return err
}

func (b *baseClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	var tools []mcp.Tool
	var cursor mcp.Cursor
	for {
		req := mcp.ListToolsRequest{}
		req.Params.Cursor = cursor
		res, err := b.inner.ListTools(ctx, req)
		if err != nil {
			return nil, err
		}
		tools = append(tools, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return tools, nil
}

func (b *baseClient) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return b.inner.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

func (b *baseClient) Close() error {
	return b.inner.Close()
}
