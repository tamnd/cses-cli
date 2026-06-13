// Package cses is the library behind the cses command line:
// the HTTP client, request shaping, and typed data models for
// the CSES Problem Set (https://cses.fi/problemset/).
//
// The Client fetches the problem list page and parses problems and their
// categories using only the standard library. No key or account is required.
package cses

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to cses.fi.
const DefaultUserAgent = "cses/dev (+https://github.com/tamnd/cses-cli)"

// Config holds constructor parameters for Client.
type Config struct {
	// BaseURL is the root of the CSES site. Override in tests.
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://cses.fi",
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to cses.fi over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultConfig().BaseURL
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// Problem is one problem in the CSES Problem Set.
type Problem struct {
	// Rank is the 1-based position across all categories.
	Rank     int    `json:"rank"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

// Problems fetches /problemset/list and returns all problems. If limit > 0 at
// most that many results are returned.
func (c *Client) Problems(ctx context.Context, limit int) ([]Problem, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/problemset/list"
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch problem list: %w", err)
	}
	problems := parseProblems(string(body), strings.TrimRight(c.cfg.BaseURL, "/"))
	if limit > 0 && limit < len(problems) {
		problems = problems[:limit]
	}
	return problems, nil
}

// Search returns problems whose title or category contains query
// (case-insensitive). If limit > 0 at most that many matches are returned.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Problem, error) {
	all, err := c.Problems(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Problem
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Title), q) ||
			strings.Contains(strings.ToLower(p.Category), q) {
			out = append(out, p)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// parseProblems parses the CSES problem list HTML and returns all problems.
// It uses only stdlib string scanning — no external HTML parser.
//
// The page structure looks like:
//
//	<h2>Introductory Problems</h2>
//	<ul class="task-list">
//	  <li class="task"><a href="/problemset/task/1068">Weird Algorithm</a>...
//	  <li class="task"><a href="/problemset/task/1083">Missing Number</a>...
//	</ul>
//	<h2>Sorting and Searching</h2>
//	<ul class="task-list">...
func parseProblems(html, baseURL string) []Problem {
	base := strings.TrimRight(baseURL, "/")

	var problems []Problem
	rank := 1
	currentCategory := ""

	rest := html
	for {
		// Look for either <h2> (new category) or <li class="task"> (a problem)
		h2Idx := strings.Index(rest, "<h2>")
		taskIdx := strings.Index(rest, `<li class="task">`)

		if h2Idx < 0 && taskIdx < 0 {
			break
		}

		// Process whichever comes first
		if h2Idx >= 0 && (taskIdx < 0 || h2Idx < taskIdx) {
			// Found a category header
			rest = rest[h2Idx+4:] // skip "<h2>"
			end := strings.Index(rest, "</h2>")
			if end < 0 {
				break
			}
			currentCategory = strings.TrimSpace(stripTags(rest[:end]))
			rest = rest[end+5:]
			continue
		}

		// Found a task item
		rest = rest[taskIdx+len(`<li class="task">`):]

		// Extract the <a href="...">Title</a>
		aStart := strings.Index(rest, "<a ")
		if aStart < 0 {
			continue
		}
		tagEnd := strings.Index(rest[aStart:], ">")
		if tagEnd < 0 {
			continue
		}
		openTag := rest[aStart : aStart+tagEnd+1]
		href := extractAttr(openTag, "href")

		// We only want /problemset/task/NNNN links
		if !strings.HasPrefix(href, "/problemset/task/") {
			continue
		}

		id := strings.TrimPrefix(href, "/problemset/task/")

		// Get the link text (the problem title)
		afterTag := rest[aStart+tagEnd+1:]
		closeA := strings.Index(afterTag, "</a>")
		if closeA < 0 {
			continue
		}
		title := strings.TrimSpace(stripTags(afterTag[:closeA]))
		rest = afterTag[closeA+4:]

		if title == "" || id == "" {
			continue
		}

		problems = append(problems, Problem{
			Rank:     rank,
			ID:       id,
			Title:    title,
			Category: currentCategory,
			URL:      base + href,
		})
		rank++
	}
	return problems
}

// extractAttr returns the value of the named attribute from an HTML open tag.
func extractAttr(tag, name string) string {
	needle := name + `="`
	i := strings.Index(strings.ToLower(tag), needle)
	if i < 0 {
		needle = name + `='`
		i = strings.Index(strings.ToLower(tag), needle)
		if i < 0 {
			return ""
		}
	}
	start := i + len(needle)
	quote := tag[i+len(name)+1]
	end := strings.IndexByte(tag[start:], quote)
	if end < 0 {
		return ""
	}
	return tag[start : start+end]
}

// stripTags removes HTML tags from s and decodes common entities.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	return strings.TrimSpace(out)
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
