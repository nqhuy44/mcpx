package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/nqhuy44/mcpx/proxy/internal/admin"
	"github.com/nqhuy44/mcpx/proxy/internal/config"
	"github.com/nqhuy44/mcpx/proxy/internal/gateway"
	"github.com/nqhuy44/mcpx/proxy/internal/metrics"
)

// defaultConfig returns gateway.yaml next to the binary, falling back to cwd.
func defaultConfig() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "gateway.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "gateway.yaml"
}

func main() {
	cfgPath := defaultConfig()
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	collector := metrics.New()

	gw := gateway.New(cfg, collector)
	if err := gw.Init(context.Background()); err != nil {
		log.Fatalf("gateway init: %v", err)
	}

	// Start admin dashboard on a separate port (non-blocking).
	adminAddr := fmt.Sprintf(":%d", cfg.AdminPort)
	adminSrv := admin.New(collector)
	go func() {
		log.Printf("mcpx-proxy admin dashboard: http://localhost%s/ui", adminAddr)
		if err := http.ListenAndServe(adminAddr, adminSrv.Handler()); err != nil {
			log.Printf("admin server error: %v", err)
		}
	}()

	s := gw.BuildMCPServer()

	switch cfg.Transport {
	case "http":
		addr := fmt.Sprintf(":%d", cfg.Port)
		httpSrv := mcpserver.NewStreamableHTTPServer(s)
		log.Printf("mcpx-proxy listening on %s (HTTP)", addr)
		if err := http.ListenAndServe(addr, httpSrv); err != nil {
			log.Fatalf("http serve: %v", err)
		}
	default:
		if err := mcpserver.ServeStdio(s); err != nil {
			log.Fatalf("stdio serve: %v", err)
		}
	}
}
