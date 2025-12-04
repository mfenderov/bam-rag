package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
)

// Config holds embeddings client configuration.
type Config struct {
	SocketPath string // Unix socket path for Docker Model Runner
	Model      string // Model name (e.g., "ai/embeddinggemma")
}

// Client wraps the Docker Model Runner embeddings API.
type Client struct {
	httpClient *http.Client
	model      string
}

// New creates a new embeddings client.
func New(config Config) (*Client, error) {
	if config.SocketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", config.SocketPath)
		},
	}

	return &Client{
		httpClient: &http.Client{Transport: transport},
		model:      config.Model,
	}, nil
}

// embeddingRequest is the request payload for the embeddings API.
type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// embeddingResponse is the response from the embeddings API.
type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MaxInputChars limits input to stay within model context window.
// qwen3-embedding supports ~24000 chars (~6000 tokens).
// Using 20000 for safety margin.
const MaxInputChars = 20000

// Embed generates an embedding vector for the given text.
// Text exceeding MaxInputChars is truncated from the end.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	originalLen := len(text)
	// Truncate to avoid context window overflow
	if len(text) > MaxInputChars {
		text = text[:MaxInputChars]
	}
	slog.Debug("generating embedding", "original_len", originalLen, "truncated_len", len(text))

	req := embeddingRequest{Model: c.model, Input: text}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost/exp/vDD4.40/engines/llama.cpp/v1/embeddings",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", embResp.Error.Message)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embResp.Data[0].Embedding, nil
}

// Dimensions returns the expected embedding dimensions for common models.
func Dimensions(model string) int {
	switch model {
	case "ai/embeddinggemma":
		return 768
	case "ai/snowflake-arctic-embed":
		return 1024
	case "ai/qwen3-embedding":
		return 2560
	default:
		return 768 // default assumption
	}
}
