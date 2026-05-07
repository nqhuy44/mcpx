package transport

import (
	"context"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
)

func newHTTPClient(cfg config.ServerConfig) (*baseClient, error) {
	inner, err := mcpclient.NewStreamableHttpClient(cfg.Address)
	if err != nil {
		return nil, err
	}
	// StreamableHttpClient does not auto-start; the transport handshake must
	// be established before the first RPC call.
	if err := inner.Start(context.Background()); err != nil {
		return nil, err
	}
	return &baseClient{inner: inner}, nil
}
