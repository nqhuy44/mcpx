// Package github wraps the GitHub REST API for PR operations.
// All responses are pre-filtered — only fields useful to an LLM are returned.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

func New(token string) *Client {
	return &Client{
		token:   token,
		baseURL: "https://api.github.com",
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var ghErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &ghErr)
		return fmt.Errorf("github %d: %s", resp.StatusCode, ghErr.Message)
	}
	return json.Unmarshal(body, out)
}

func (c *Client) post(path, body string) error {
	req, err := http.NewRequest("POST", c.baseURL+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var ghErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &ghErr)
		return fmt.Errorf("github %d: %s", resp.StatusCode, ghErr.Message)
	}
	return nil
}

// PR is a token-lean PR summary.
type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Author string `json:"author"`
	Base   string `json:"base"`
	Head   string `json:"head"`
	URL    string `json:"url"`
	Draft  bool   `json:"draft,omitempty"`
	Labels []string `json:"labels,omitempty"`
	CreatedAt string `json:"created_at"`
}

type rawPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Draft  bool   `json:"draft"`
	HTMLURL string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	User  struct{ Login string `json:"login"` } `json:"user"`
	Base  struct{ Ref string `json:"ref"` } `json:"base"`
	Head  struct{ Ref string `json:"ref"` } `json:"head"`
	Labels []struct{ Name string `json:"name"` } `json:"labels"`
}

func (r rawPR) lean() PR {
	labels := make([]string, 0, len(r.Labels))
	for _, l := range r.Labels {
		labels = append(labels, l.Name)
	}
	return PR{
		Number:    r.Number,
		Title:     r.Title,
		State:     r.State,
		Author:    r.User.Login,
		Base:      r.Base.Ref,
		Head:      r.Head.Ref,
		URL:       r.HTMLURL,
		Draft:     r.Draft,
		Labels:    labels,
		CreatedAt: r.CreatedAt[:10],
	}
}

// PRList returns open PRs for owner/repo, optionally filtered by state.
func (c *Client) PRList(owner, repo, state string) ([]PR, error) {
	if state == "" {
		state = "open"
	}
	var raw []rawPR
	if err := c.get(fmt.Sprintf("/repos/%s/%s/pulls?state=%s&per_page=30", owner, repo, state), &raw); err != nil {
		return nil, err
	}
	out := make([]PR, len(raw))
	for i, r := range raw {
		out[i] = r.lean()
	}
	return out, nil
}

// PRDetail is a PR with filtered diff stats.
type PRDetail struct {
	PR
	Body       string `json:"body,omitempty"`
	ChangedFiles int  `json:"changed_files"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Mergeable  *bool  `json:"mergeable"`
}

// PRGet returns details for a single PR number.
func (c *Client) PRGet(owner, repo string, number int) (*PRDetail, error) {
	var raw struct {
		rawPR
		Body         string `json:"body"`
		ChangedFiles int    `json:"changed_files"`
		Additions    int    `json:"additions"`
		Deletions    int    `json:"deletions"`
		Mergeable    *bool  `json:"mergeable"`
	}
	if err := c.get(fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), &raw); err != nil {
		return nil, err
	}
	// Trim body to 500 chars to avoid bloating context.
	body := raw.Body
	if len(body) > 500 {
		body = body[:500] + "…"
	}
	return &PRDetail{
		PR:           raw.rawPR.lean(),
		Body:         body,
		ChangedFiles: raw.ChangedFiles,
		Additions:    raw.Additions,
		Deletions:    raw.Deletions,
		Mergeable:    raw.Mergeable,
	}, nil
}

// PRComment posts a review comment on a PR.
func (c *Client) PRComment(owner, repo string, number int, body string) error {
	payload := fmt.Sprintf(`{"body":%s}`, jsonString(body))
	return c.post(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), payload)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
