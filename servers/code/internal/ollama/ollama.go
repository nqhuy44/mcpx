package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	url  string
	opts map[string]any

	// explicit model name; empty means auto-detect from /api/ps on first call
	model string
	mu    sync.Mutex
	autoModel string
}

func New() *Client {
	opts := map[string]any{
		"num_ctx": envInt("OLLAMA_NUM_CTX", 16384),
	}
	if v, ok := envFloat("OLLAMA_TEMPERATURE"); ok {
		opts["temperature"] = v
	}
	if v, ok := envFloat("OLLAMA_TOP_P"); ok {
		opts["top_p"] = v
	}
	if v, ok := envInt2("OLLAMA_TOP_K"); ok {
		opts["top_k"] = v
	}

	return &Client{
		url:   envStr("OLLAMA_URL", "http://localhost:11434"),
		model: os.Getenv("OLLAMA_MODEL"), // empty = auto-detect
		opts:  opts,
	}
}

func (c *Client) Generate(prompt string) (string, error) {
	model, err := c.resolveModel()
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]any{
		"model":   model,
		"prompt":  prompt,
		"stream":  false,
		"options": c.opts,
	})

	hc := &http.Client{Timeout: 120 * time.Second}
	resp, err := hc.Post(c.url+"/api/generate", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ollama unavailable at %s: %w", c.url, err)
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama: %s", result.Error)
	}
	return strings.TrimSpace(result.Response), nil
}

// resolveModel returns the model name to use. If OLLAMA_MODEL is set, it is
// used directly. Otherwise the first model currently loaded in Ollama (/api/ps)
// is used and cached for subsequent calls.
func (c *Client) resolveModel() (string, error) {
	if c.model != "" {
		return c.model, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.autoModel != "" {
		return c.autoModel, nil
	}

	hc := &http.Client{Timeout: 3 * time.Second}
	resp, err := hc.Get(c.url + "/api/ps")
	if err != nil {
		return "", fmt.Errorf("ollama unavailable at %s (set OLLAMA_MODEL to override): %w", c.url, err)
	}
	defer resp.Body.Close()

	var ps struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil || len(ps.Models) == 0 {
		return "", fmt.Errorf("no model loaded in Ollama — run 'ollama run <model>' or set OLLAMA_MODEL in gateway.yaml")
	}

	c.autoModel = ps.Models[0].Name
	return c.autoModel, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt2(key string) (int, bool) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n, true
		}
	}
	return 0, false
}

func envFloat(key string) (float64, bool) {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
