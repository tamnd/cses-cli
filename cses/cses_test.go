package cses_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/cses-cli/cses"
)

// minimalPage returns a minimal CSES-like problem list page with the given
// categories and problems for use in httptest servers.
func minimalPage(sections map[string][]struct{ id, title string }) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body>`)
	for cat, problems := range sections {
		fmt.Fprintf(&sb, "<h2>%s</h2><ul class=\"task-list\">", cat)
		for _, p := range problems {
			fmt.Fprintf(&sb,
				`<li class="task"><a href="/problemset/task/%s">%s</a><span class="detail">100 / 200</span></li>`,
				p.id, p.title)
		}
		sb.WriteString("</ul>")
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func TestProblems(t *testing.T) {
	page := minimalPage(map[string][]struct{ id, title string }{
		"Introductory Problems": {
			{"1068", "Weird Algorithm"},
			{"1083", "Missing Number"},
		},
		"Sorting and Searching": {
			{"1621", "Distinct Numbers"},
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/problemset/list" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := cses.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := cses.NewClient(cfg)

	problems, err := client.Problems(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(problems) != 3 {
		t.Errorf("got %d problems, want 3", len(problems))
	}

	// The page is deterministic in iteration order only if we control the map.
	// Just check that all three titles appear.
	titles := make(map[string]bool, len(problems))
	for _, p := range problems {
		titles[p.Title] = true
		if p.URL == "" {
			t.Errorf("problem %q has empty URL", p.Title)
		}
		if p.ID == "" {
			t.Errorf("problem %q has empty ID", p.Title)
		}
		if p.Category == "" {
			t.Errorf("problem %q has empty Category", p.Title)
		}
		if p.Rank == 0 {
			t.Errorf("problem %q has rank 0", p.Title)
		}
	}
	for _, want := range []string{"Weird Algorithm", "Missing Number", "Distinct Numbers"} {
		if !titles[want] {
			t.Errorf("missing problem %q", want)
		}
	}
}

func TestProblemsLimit(t *testing.T) {
	page := minimalPage(map[string][]struct{ id, title string }{
		"Introductory Problems": {
			{"1068", "Weird Algorithm"},
			{"1083", "Missing Number"},
			{"1069", "Repetitions"},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := cses.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := cses.NewClient(cfg)

	problems, err := client.Problems(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 2 {
		t.Errorf("got %d problems with limit=2, want 2", len(problems))
	}
}

func TestSearch(t *testing.T) {
	page := minimalPage(map[string][]struct{ id, title string }{
		"Sorting and Searching": {
			{"1621", "Distinct Numbers"},
			{"1084", "Apartments"},
		},
		"Dynamic Programming": {
			{"1633", "Dice Combinations"},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := cses.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := cses.NewClient(cfg)

	// Title match
	hits, err := client.Search(context.Background(), "distinct", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "Distinct Numbers" {
		t.Errorf("search 'distinct': got %v", hits)
	}

	// Category match
	hits, err = client.Search(context.Background(), "sorting", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Errorf("search 'sorting': got %d hits, want 2", len(hits))
	}
}

func TestSearchLimit(t *testing.T) {
	page := minimalPage(map[string][]struct{ id, title string }{
		"Sorting and Searching": {
			{"1621", "Distinct Numbers"},
			{"1084", "Apartments"},
			{"2162", "Josephus Problem I"},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := cses.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := cses.NewClient(cfg)

	hits, err := client.Search(context.Background(), "sorting", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Errorf("search with limit=2: got %d hits, want 2", len(hits))
	}
}

func TestProblemURL(t *testing.T) {
	page := minimalPage(map[string][]struct{ id, title string }{
		"Introductory Problems": {
			{"1068", "Weird Algorithm"},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := cses.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := cses.NewClient(cfg)

	problems, err := client.Problems(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1", len(problems))
	}
	p := problems[0]
	wantURL := srv.URL + "/problemset/task/1068"
	if p.URL != wantURL {
		t.Errorf("URL = %q, want %q", p.URL, wantURL)
	}
	if p.ID != "1068" {
		t.Errorf("ID = %q, want %q", p.ID, "1068")
	}
}
