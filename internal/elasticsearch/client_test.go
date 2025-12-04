package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mfenderov/bam-rag/pkg/models"
)

func skipIfNoES(t *testing.T) {
	if os.Getenv("SKIP_ES_TESTS") == "1" {
		t.Skip("Skipping ES tests (SKIP_ES_TESTS=1)")
	}

	// Try to connect to ES
	client, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "test-skip-check",
	})
	if err != nil {
		t.Skipf("Skipping ES tests: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if !client.Ping(ctx) {
		t.Skip("Skipping ES tests: Elasticsearch not available")
	}
}

func TestClient_Connect(t *testing.T) {
	skipIfNoES(t)

	client, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-test",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	if !client.Ping(ctx) {
		t.Error("Ping() should return true for running ES")
	}
}

func TestClient_CreateIndex(t *testing.T) {
	skipIfNoES(t)

	client, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-test-create",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Delete index if exists (cleanup from previous test)
	client.DeleteIndex(ctx)

	// Create index
	err = client.CreateIndex(ctx)
	if err != nil {
		t.Fatalf("CreateIndex() error = %v", err)
	}

	// Creating again should not error (idempotent)
	err = client.CreateIndex(ctx)
	if err != nil {
		t.Fatalf("CreateIndex() second call error = %v", err)
	}

	// Cleanup
	client.DeleteIndex(ctx)
}

func TestClient_IndexAndSearch(t *testing.T) {
	skipIfNoES(t)

	client, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-test-search",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Setup: delete and create fresh index
	client.DeleteIndex(ctx)
	if err := client.CreateIndex(ctx); err != nil {
		t.Fatalf("CreateIndex() error = %v", err)
	}

	// Index some documents
	docs := []models.Document{
		{
			ID:      "doc1",
			URL:     "https://example.com/docs/install",
			Title:   "Installation Guide",
			Content: "# Installation\n\nRun go install to install the package.",
		},
		{
			ID:      "doc2",
			URL:     "https://example.com/docs/config",
			Title:   "Configuration Guide",
			Content: "# Configuration\n\nConfigure the application using environment variables.",
		},
		{
			ID:      "doc3",
			URL:     "https://example.com/api/users",
			Title:   "Users API",
			Content: "# Users API\n\nThe users endpoint returns a list of all users.",
		},
	}

	for _, doc := range docs {
		if err := client.IndexDocument(ctx, doc); err != nil {
			t.Fatalf("IndexDocument() error = %v", err)
		}
	}

	// Wait for ES to index (refresh)
	time.Sleep(1 * time.Second)
	client.Refresh(ctx)

	// Search for "install"
	results, err := client.Search(ctx, "install", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Search('install') should return results")
	}

	// First result should be the installation doc
	found := false
	for _, r := range results {
		if r.ID == "doc1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Search results should include doc1 (installation)")
	}

	// Search for "users" should return API doc
	results, err = client.Search(ctx, "users", 10)
	if err != nil {
		t.Fatalf("Search('users') error = %v", err)
	}

	found = false
	for _, r := range results {
		if r.ID == "doc3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Search('users') should include doc3")
	}

	// Cleanup
	client.DeleteIndex(ctx)
}

func TestClient_GetDocument(t *testing.T) {
	skipIfNoES(t)

	client, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
		Index:     "bam-rag-test-get",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Setup
	client.DeleteIndex(ctx)
	client.CreateIndex(ctx)

	doc := models.Document{
		ID:      "test-doc-get",
		URL:     "https://example.com/test",
		Title:   "Test Page",
		Content: "# Test\n\nTest content for get operation.",
	}

	if err := client.IndexDocument(ctx, doc); err != nil {
		t.Fatalf("IndexDocument() error = %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Get the document
	result, err := client.GetDocument(ctx, "test-doc-get")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetDocument() returned nil")
	}

	if result.ID != doc.ID {
		t.Errorf("ID = %q, want %q", result.ID, doc.ID)
	}
	if result.Content != doc.Content {
		t.Errorf("Content mismatch")
	}

	// Cleanup
	client.DeleteIndex(ctx)
}
