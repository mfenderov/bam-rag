package mcp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/mfenderov/bam-rag/pkg/models"
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

func TestServer_Creation(t *testing.T) {
	s, err := NewServer(Config{
		Name:        "bam-rag",
		Version:     "1.0.0",
		ESAddresses: []string{"http://localhost:9200"},
		ESIndex:     "bam-rag-test",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if s == nil {
		t.Fatal("NewServer() returned nil")
	}

	if s.mcpServer == nil {
		t.Error("mcpServer should not be nil")
	}
}

func TestServer_SearchTool(t *testing.T) {
	skipIfNoES(t)

	ctx := context.Background()

	// Setup ES with test data
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-mcp-test",
	})
	if err != nil {
		t.Fatalf("Failed to create ES client: %v", err)
	}

	// Setup test data
	esClient.DeleteIndex(ctx)
	esClient.CreateIndex(ctx)

	docs := []models.Document{
		{
			ID:      "mcp-test-1",
			URL:     "https://example.com/docs",
			Title:   "Documentation",
			Content: "# Getting Started\n\nWelcome to the getting started guide for installation.",
		},
		{
			ID:      "mcp-test-2",
			URL:     "https://example.com/api",
			Title:   "API Reference",
			Content: "# API Endpoints\n\nThe API provides RESTful endpoints for users.",
		},
	}

	for _, doc := range docs {
		esClient.IndexDocument(ctx, doc)
	}
	time.Sleep(1 * time.Second)
	esClient.Refresh(ctx)

	// Create server
	s, err := NewServer(Config{
		Name:        "bam-rag",
		Version:     "1.0.0",
		ESAddresses: []string{"http://localhost:9200"},
		ESIndex:     "bam-rag-mcp-test",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Test search handler directly
	results, err := s.handleSearch(ctx, "installation", 10)
	if err != nil {
		t.Fatalf("handleSearch() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("handleSearch() should return results for 'installation'")
	}

	// Cleanup
	esClient.DeleteIndex(ctx)
}

func TestServer_GetDocumentTool(t *testing.T) {
	skipIfNoES(t)

	ctx := context.Background()

	// Setup ES with test data
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-mcp-get-test",
	})
	if err != nil {
		t.Fatalf("Failed to create ES client: %v", err)
	}

	// Setup test data
	esClient.DeleteIndex(ctx)
	esClient.CreateIndex(ctx)

	doc := models.Document{
		ID:      "mcp-get-test",
		URL:     "https://example.com/test",
		Title:   "Test Page",
		Content: "# Test\n\nTest content for MCP get document.",
	}
	esClient.IndexDocument(ctx, doc)
	time.Sleep(500 * time.Millisecond)

	// Create server
	s, err := NewServer(Config{
		Name:        "bam-rag",
		Version:     "1.0.0",
		ESAddresses: []string{"http://localhost:9200"},
		ESIndex:     "bam-rag-mcp-get-test",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Test get handler directly
	result, err := s.handleGetDocument(ctx, "mcp-get-test")
	if err != nil {
		t.Fatalf("handleGetDocument() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleGetDocument() returned nil")
	}

	if result.ID != doc.ID {
		t.Errorf("ID = %q, want %q", result.ID, doc.ID)
	}

	// Cleanup
	esClient.DeleteIndex(ctx)
}
