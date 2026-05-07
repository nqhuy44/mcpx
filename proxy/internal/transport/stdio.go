package transport

import (
	"os"
	"path/filepath"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
)

func newStdioClient(cfg config.ServerConfig) (*baseClient, error) {
	binary := resolveBinary(cfg.Binary, cfg.ConfigDir)
	inner, err := mcpclient.NewStdioMCPClient(binary, mergeEnv(cfg.Env))
	if err != nil {
		return nil, err
	}
	return &baseClient{inner: inner}, nil
}

// mergeEnv merges the current process environment with per-server overrides.
// Returns nil (inherit unchanged) when overrides is empty.
func mergeEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}
	base := make(map[string]string, len(os.Environ()))
	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		base[k] = v
	}
	for k, v := range overrides {
		base[k] = v
	}
	env := make([]string, 0, len(base))
	for k, v := range base {
		env = append(env, k+"="+v)
	}
	return env
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
