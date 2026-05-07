package admin

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/nqhuy44/mcpx/proxy/internal/metrics"
)

//go:embed ui/index.html
var indexHTML []byte

// Server is the HTTP admin server that exposes the dashboard and status API.
type Server struct {
	collector *metrics.Collector
	mux       *http.ServeMux
}

func New(collector *metrics.Collector) *Server {
	s := &Server{collector: collector, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/ui", s.handleUI)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui", http.StatusFound)
	})
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.collector.Snapshot()) //nolint:errcheck
}

func (s *Server) handleUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML) //nolint:errcheck
}
