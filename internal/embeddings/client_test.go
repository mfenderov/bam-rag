package embeddings

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "empty socket path",
			config:  Config{SocketPath: "", Model: "test-model"},
			wantErr: true,
		},
		{
			name:    "empty model",
			config:  Config{SocketPath: "/tmp/test.sock", Model: ""},
			wantErr: true,
		},
		{
			name:    "valid config",
			config:  Config{SocketPath: "/tmp/test.sock", Model: "test-model"},
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

func TestDimensions(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"ai/embeddinggemma", 768},
		{"ai/snowflake-arctic-embed", 1024},
		{"ai/qwen3-embedding", 2560},
		{"unknown-model", 768}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := Dimensions(tt.model); got != tt.want {
				t.Errorf("Dimensions(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestEmbed_Success(t *testing.T) {
	// Create a mock server on a Unix socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}
	defer listener.Close()

	// Mock response
	mockEmbedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	mockResponse := embeddingResponse{
		Data: []struct {
			Embedding []float32 `json:"embedding"`
		}{
			{Embedding: mockEmbedding},
		},
	}

	// Start mock server
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Expected application/json content type")
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	// Create client and test
	client, err := New(Config{
		SocketPath: socketPath,
		Model:      "test-model",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	embedding, err := client.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != len(mockEmbedding) {
		t.Errorf("Embed() returned %d dimensions, want %d", len(embedding), len(mockEmbedding))
	}

	for i, v := range embedding {
		if v != mockEmbedding[i] {
			t.Errorf("Embed()[%d] = %v, want %v", i, v, mockEmbedding[i])
		}
	}
}

func TestEmbed_ServerError(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	client, err := New(Config{
		SocketPath: socketPath,
		Model:      "test-model",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.Embed(context.Background(), "test text")
	if err == nil {
		t.Error("Embed() expected error for server error response")
	}
}

func TestEmbed_EmptyResponse(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}
	defer listener.Close()

	// Empty data response
	mockResponse := embeddingResponse{Data: []struct {
		Embedding []float32 `json:"embedding"`
	}{}}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	client, err := New(Config{
		SocketPath: socketPath,
		Model:      "test-model",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.Embed(context.Background(), "test text")
	if err == nil {
		t.Error("Embed() expected error for empty response")
	}
}

// Skip integration test if DMR is not available
func TestEmbed_Integration(t *testing.T) {
	socketPath := os.Getenv("DOCKER_SOCKET")
	if socketPath == "" {
		socketPath = os.ExpandEnv("$HOME/.docker/run/docker.sock")
	}

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skip("Docker socket not available, skipping integration test")
	}

	client, err := New(Config{
		SocketPath: socketPath,
		Model:      "ai/embeddinggemma",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	embedding, err := client.Embed(context.Background(), "Hello, this is a test")
	if err != nil {
		t.Skipf("DMR not available or model not pulled: %v", err)
	}

	// embeddinggemma should return 768 dimensions
	if len(embedding) != 768 {
		t.Errorf("Expected 768 dimensions, got %d", len(embedding))
	}
}

// Unused but kept for reference
var _ = httptest.NewServer
