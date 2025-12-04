package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Document represents a scraped web page.
type Document struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"` // HTTP Content-Type header
	ScrapedAt   time.Time `json:"scraped_at"`
	Tags        []string  `json:"tags,omitempty"`      // LLM-generated search keywords
	Summary     string    `json:"summary,omitempty"`   // LLM-generated summary
	Embedding   []float32 `json:"embedding,omitempty"` // Vector embedding of summary
}

// GenerateDocumentID creates a deterministic ID from URL.
// The ID is a SHA-256 hash (first 16 chars) of the URL.
func GenerateDocumentID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])[:16]
}
