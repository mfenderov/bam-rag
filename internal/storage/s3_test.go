package storage

import (
	"context"
	"os"
	"testing"
)

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "empty endpoint",
			config:  Config{Endpoint: "", Bucket: "test"},
			wantErr: true,
		},
		{
			name:    "empty bucket",
			config:  Config{Endpoint: "localhost:9000", Bucket: ""},
			wantErr: true,
		},
		{
			name: "valid config",
			config: Config{
				Endpoint:        "localhost:9000",
				Bucket:          "test",
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestIntegration_S3Operations tests actual S3 operations against MinIO.
// Skip if MinIO is not running.
func TestIntegration_S3Operations(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}

	client, err := New(Config{
		Endpoint:        endpoint,
		Bucket:          "bam-rag-test",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UseSSL:          false,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Try to ensure bucket - skip if MinIO is not available
	if err := client.EnsureBucket(ctx); err != nil {
		t.Skipf("MinIO not available, skipping integration test: %v", err)
	}

	// Test prefix for this run
	prefix := "scrapes/test.example.com/2024-12-04T17-30-00-test123"

	// Test PutMarkdown
	t.Run("PutMarkdown", func(t *testing.T) {
		content := "# Test Page\n\nThis is test content."
		err := client.PutMarkdown(ctx, prefix, "abc123.md", content)
		if err != nil {
			t.Fatalf("PutMarkdown() error = %v", err)
		}
	})

	// Test GetMarkdown
	t.Run("GetMarkdown", func(t *testing.T) {
		content, err := client.GetMarkdown(ctx, prefix, "abc123.md")
		if err != nil {
			t.Fatalf("GetMarkdown() error = %v", err)
		}
		expected := "# Test Page\n\nThis is test content."
		if content != expected {
			t.Errorf("GetMarkdown() = %q, want %q", content, expected)
		}
	})

	// Test PutMetadata
	t.Run("PutMetadata", func(t *testing.T) {
		meta := ScrapeMetadata{
			SourceURL: "https://test.example.com/docs",
			Timestamp: "2024-12-04T17:30:00Z",
			PageCount: 1,
			Pages:     []string{"https://test.example.com/docs/page1"},
		}
		err := client.PutMetadata(ctx, prefix, meta)
		if err != nil {
			t.Fatalf("PutMetadata() error = %v", err)
		}
	})

	// Test GetMetadata
	t.Run("GetMetadata", func(t *testing.T) {
		meta, err := client.GetMetadata(ctx, prefix)
		if err != nil {
			t.Fatalf("GetMetadata() error = %v", err)
		}
		if meta.SourceURL != "https://test.example.com/docs" {
			t.Errorf("GetMetadata().SourceURL = %q, want %q", meta.SourceURL, "https://test.example.com/docs")
		}
		if meta.PageCount != 1 {
			t.Errorf("GetMetadata().PageCount = %d, want %d", meta.PageCount, 1)
		}
	})

	// Test ListMarkdownFiles
	t.Run("ListMarkdownFiles", func(t *testing.T) {
		files, err := client.ListMarkdownFiles(ctx, prefix)
		if err != nil {
			t.Fatalf("ListMarkdownFiles() error = %v", err)
		}
		if len(files) != 1 {
			t.Errorf("ListMarkdownFiles() returned %d files, want 1", len(files))
		}
		if len(files) > 0 && files[0] != "abc123.md" {
			t.Errorf("ListMarkdownFiles()[0] = %q, want %q", files[0], "abc123.md")
		}
	})
}
