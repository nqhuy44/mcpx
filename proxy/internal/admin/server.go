package admin

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/nqhuy44/mcpx/proxy/internal/metrics"
)

//go:embed ui/index.html
var indexHTML []byte

// Toggler is implemented by *gateway.Gateway. Defined here to avoid an
// import cycle (admin → gateway → transport → ...).
type Toggler interface {
	DisableServer(name string)
	EnableServer(ctx context.Context, name string) error
}

// Server is the HTTP admin server that exposes the dashboard and status API.
type Server struct {
	collector *metrics.Collector
	toggler   Toggler
	mux       *http.ServeMux
}

func New(collector *metrics.Collector, toggler Toggler) *Server {
	s := &Server{collector: collector, toggler: toggler, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/servers/{name}/disable", s.handleDisable)
	s.mux.HandleFunc("POST /api/servers/{name}/enable", s.handleEnable)
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

func (s *Server) handleDisable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.toggler.DisableServer(name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled", "server": name}) //nolint:errcheck
}

func (s *Server) handleEnable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.toggler.EnableServer(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "server": name}) //nolint:errcheck
}

func (s *Server) handleUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML) //nolint:errcheck
}
