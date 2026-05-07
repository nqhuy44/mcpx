package transport

import (
	"os"
	"path/filepath"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
)

func newStdioClient(cfg config.ServerConfig) (*baseClient, error) {
	binary := resolveBinary(cfg.Binary, cfg.ConfigDir)
	inner, err := mcpclient.NewStdioMCPClient(binary, nil)
	if err != nil {
		return nil, err
	}
	return &baseClient{inner: inner}, nil
}

// resolveBinary turns a relative binary path into an absolute one.
// Relative paths are resolved against the config file's directory so that
// "binary: bin/mcpx-git" works regardless of the working directory the
// proxy binary was launched from.
func resolveBinary(binary, configDir string) string {
	if filepath.IsAbs(binary) {
		return binary
	}
	if configDir != "" {
		abs := filepath.Join(configDir, binary)
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	return binary
}
