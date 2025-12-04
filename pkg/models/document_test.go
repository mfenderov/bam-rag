package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDocument_JSONSerialization(t *testing.T) {
	// Arrange
	doc := Document{
		URL:       "https://example.com/docs/intro",
		Title:     "Introduction",
		Content:   "# Introduction\n\nWelcome to the docs.",
		ScrapedAt: time.Date(2025, 12, 4, 10, 0, 0, 0, time.UTC),
	}

	// Act - serialize to JSON
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal Document: %v", err)
	}

	// Act - deserialize from JSON
	var decoded Document
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Document: %v", err)
	}

	// Assert
	if decoded.URL != doc.URL {
		t.Errorf("URL mismatch: got %q, want %q", decoded.URL, doc.URL)
	}
	if decoded.Title != doc.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, doc.Title)
	}
	if decoded.Content != doc.Content {
		t.Errorf("Content mismatch: got %q, want %q", decoded.Content, doc.Content)
	}
	if !decoded.ScrapedAt.Equal(doc.ScrapedAt) {
		t.Errorf("ScrapedAt mismatch: got %v, want %v", decoded.ScrapedAt, doc.ScrapedAt)
	}
}

func TestDocument_JSONFieldNames(t *testing.T) {
	doc := Document{
		URL:       "https://example.com",
		Title:     "Test",
		Content:   "content",
		ScrapedAt: time.Now(),
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify JSON uses snake_case field names
	jsonStr := string(data)
	expectedFields := []string{`"url"`, `"title"`, `"content"`, `"scraped_at"`}
	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON should contain field %s, got: %s", field, jsonStr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateDocumentID(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"simple URL", "https://example.com/docs"},
		{"URL with path", "https://example.com/docs/intro/getting-started"},
		{"URL with query", "https://example.com/docs?page=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateDocumentID(tt.url)

			// ID should not be empty
			if id == "" {
				t.Error("ID should not be empty")
			}

			// ID should be deterministic
			id2 := GenerateDocumentID(tt.url)
			if id != id2 {
				t.Errorf("ID should be deterministic: got %q and %q", id, id2)
			}

			// ID should be 16 chars (hex encoded, truncated)
			if len(id) != 16 {
				t.Errorf("ID length should be 16, got %d", len(id))
			}
		})
	}
}

func TestGenerateDocumentID_UniqueForDifferentURLs(t *testing.T) {
	url1 := "https://example.com/page1"
	url2 := "https://example.com/page2"

	id1 := GenerateDocumentID(url1)
	id2 := GenerateDocumentID(url2)

	if id1 == id2 {
		t.Errorf("Different URLs should generate different IDs: %q", id1)
	}
}
