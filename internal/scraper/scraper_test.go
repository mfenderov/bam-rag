package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScraper_FetchSingleURL(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<head><title>Test Page</title></head>
			<body>
				<h1>Hello World</h1>
				<p>This is a test page.</p>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	s := New(Config{
		Delay:     10 * time.Millisecond,
		MaxDepth:  1,
		UserAgent: "test-agent",
	})

	docs, err := s.Scrape(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	doc := docs[0]
	// URL might have trailing slash normalized
	if !strings.HasPrefix(doc.URL, server.URL) {
		t.Errorf("URL = %q, want prefix %q", doc.URL, server.URL)
	}
	if !strings.Contains(doc.Content, "Hello World") {
		t.Error("Content should contain 'Hello World'")
	}
	if doc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt should not be zero")
	}
}

func TestScraper_FollowsLinksWithinDomain(t *testing.T) {
	pages := map[string]string{
		"/": `<html><head><title>Home</title></head><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`,
		"/page1": `<html><head><title>Page 1</title></head><body>
			<h1>Page 1 Content</h1>
		</body></html>`,
		"/page2": `<html><head><title>Page 2</title></head><body>
			<h1>Page 2 Content</h1>
		</body></html>`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if content, ok := pages[r.URL.Path]; ok {
			w.Write([]byte(content))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := New(Config{
		Delay:       10 * time.Millisecond,
		MaxDepth:    2,
		FollowLinks: true,
		UserAgent:   "test-agent",
	})

	docs, err := s.Scrape(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	// Should have scraped all 3 pages
	if len(docs) < 3 {
		t.Errorf("expected at least 3 documents, got %d", len(docs))
	}

	// Check that we got different URLs
	urls := make(map[string]bool)
	for _, doc := range docs {
		urls[doc.URL] = true
	}
	if !urls[server.URL+"/page1"] {
		t.Error("should have scraped /page1")
	}
	if !urls[server.URL+"/page2"] {
		t.Error("should have scraped /page2")
	}
}

func TestScraper_RespectsMaxDepth(t *testing.T) {
	pages := map[string]string{
		"/":       `<html><body><a href="/level1">Level 1</a></body></html>`,
		"/level1": `<html><body><a href="/level2">Level 2</a></body></html>`,
		"/level2": `<html><body><a href="/level3">Level 3</a></body></html>`,
		"/level3": `<html><body>Deep content</body></html>`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if content, ok := pages[r.URL.Path]; ok {
			w.Write([]byte(content))
		}
	}))
	defer server.Close()

	s := New(Config{
		Delay:       10 * time.Millisecond,
		MaxDepth:    2, // Should only go to level1
		FollowLinks: true,
		UserAgent:   "test-agent",
	})

	docs, err := s.Scrape(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	// With MaxDepth=2, should get root and level1, but not deeper
	urls := make(map[string]bool)
	for _, doc := range docs {
		urls[doc.URL] = true
	}

	if !urls[server.URL+"/level1"] {
		t.Error("should have scraped /level1 (depth 2)")
	}
	// level2 and level3 should not be scraped
	if urls[server.URL+"/level3"] {
		t.Error("should NOT have scraped /level3 (beyond max depth)")
	}
}

func TestScraper_HandlesErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	s := New(Config{
		Delay:     10 * time.Millisecond,
		MaxDepth:  1,
		UserAgent: "test-agent",
	})

	docs, err := s.Scrape(t.Context(), server.URL)
	// Should not return error, just empty results
	if err != nil {
		t.Logf("Scrape returned error (acceptable): %v", err)
	}

	// Should have no successful documents
	if len(docs) > 0 {
		t.Errorf("expected 0 documents for error response, got %d", len(docs))
	}
}

func TestScraper_SetsUserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>Test</body></html>`))
	}))
	defer server.Close()

	s := New(Config{
		Delay:     10 * time.Millisecond,
		MaxDepth:  1,
		UserAgent: "BAM-RAG/1.0",
	})

	_, err := s.Scrape(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	if receivedUA != "BAM-RAG/1.0" {
		t.Errorf("User-Agent = %q, want %q", receivedUA, "BAM-RAG/1.0")
	}
}
