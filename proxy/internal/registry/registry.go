package registry

import (
	"encoding/json"
	"strings"
	"sync"
)

type ToolEntry struct {
	Name        string
	ServerName  string
	Description string
	InputSchema json.RawMessage
	Keywords    []string
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolEntry
}

func New() *Registry {
	return &Registry{tools: make(map[string]*ToolEntry)}
}

func (r *Registry) Register(entry *ToolEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[entry.Name] = entry
}

func (r *Registry) Get(name string) (*ToolEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.tools[name]
	return e, ok
}

// Search returns up to limit entries whose name, description, or keywords
// contain any token from the query. It is a simple keyword match — no ranking.
func (r *Registry) Search(query string, limit int) []*ToolEntry {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*ToolEntry
	for _, e := range r.tools {
		if matches(e, tokens) {
			results = append(results, e)
			if len(results) == limit {
				break
			}
		}
	}
	return results
}

func (r *Registry) All() []*ToolEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]*ToolEntry, 0, len(r.tools))
	for _, e := range r.tools {
		entries = append(entries, e)
	}
	return entries
}

func matches(e *ToolEntry, tokens []string) bool {
	haystack := strings.ToLower(e.Name + " " + e.Description + " " + strings.Join(e.Keywords, " "))
	for _, t := range tokens {
		if strings.Contains(haystack, t) {
			return true
		}
	}
	return false
}
