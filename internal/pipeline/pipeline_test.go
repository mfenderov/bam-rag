package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mfenderov/bam-rag/internal/elasticsearch"
)

func skipIfNoES(t *testing.T) {
	if os.Getenv("SKIP_ES_TESTS") == "1" {
		t.Skip("Skipping ES tests")
	}
	client, err := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "test-skip",
	})
	if err != nil {
		t.Skipf("Skipping: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if !client.Ping(ctx) {
		t.Skip("Skipping: ES not available")
	}
}

func TestPipeline_EndToEnd(t *testing.T) {
	skipIfNoES(t)

	// Create test server with HTML content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Documentation</title></head>
			<body>
				<h1>Getting Started</h1>
				<p>Welcome to the documentation.</p>

				<h2>Installation</h2>
				<p>Run the following command to install:</p>
				<pre><code>go install example.com/tool</code></pre>

				<h2>Configuration</h2>
				<p>Configure using environment variables.</p>

				<h3>Environment Variables</h3>
				<p>Set FOO_BAR to customize behavior.</p>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	// Create pipeline
	p, err := New(Config{
		ESAddresses: []string{"http://localhost:9200"},
		ESIndex:     "bam-rag-pipeline-test",
		ScraperConfig: ScraperConfig{
			Delay:       10 * time.Millisecond,
			MaxDepth:    1,
			FollowLinks: false,
			UserAgent:   "test-agent",
		},
	})
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	ctx := context.Background()

	// Clean up any existing index
	p.DeleteIndex(ctx)

	// Run pipeline
	result, err := p.Run(ctx, server.URL)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify results
	if result.PagesScraped != 1 {
		t.Errorf("PagesScraped = %d, want 1", result.PagesScraped)
	}
	if result.DocsIndexed != 1 {
		t.Errorf("DocsIndexed = %d, want 1", result.DocsIndexed)
	}

	// Wait for ES to index
	time.Sleep(1 * time.Second)

	// Search for content
	docs, err := p.Search(ctx, "install", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(docs) == 0 {
		t.Error("Search('install') should return results")
	}

	// Verify document has proper structure
	if len(docs) > 0 {
		doc := docs[0]
		if doc.ID == "" {
			t.Error("Document ID should not be empty")
		}
		if doc.URL == "" {
			t.Error("Document URL should not be empty")
		}
		if doc.Title == "" {
			t.Error("Document Title should not be empty")
		}
		if doc.Content == "" {
			t.Error("Document Content should not be empty")
		}
	}

	// Cleanup
	p.DeleteIndex(ctx)
}
