package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// Config holds LLM client configuration.
type Config struct {
	SocketPath string // Unix socket path for Docker Model Runner
	Model      string // Model name (e.g., "ai/gemma3")
}

// Client wraps the Docker Model Runner chat completions API.
type Client struct {
	httpClient *http.Client
	model      string
}

// New creates a new LLM client.
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

// chatRequest is the request payload for the chat completions API.
type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"` // Limit response length
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the response from the chat completions API.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt to the LLM and returns the response.
func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithMaxTokens(ctx, prompt, 0)
}

// CompleteWithMaxTokens sends a prompt with a token limit on the response.
// If maxTokens is 0, no limit is applied.
func (c *Client) CompleteWithMaxTokens(ctx context.Context, prompt string, maxTokens int) (string, error) {
	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: maxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost/exp/vDD4.40/engines/llama.cpp/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response returned")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// EnrichmentResult holds the generated tags and summary.
type EnrichmentResult struct {
	Tags    []string
	Summary string
}

// MaxContentForEnrichment limits content sent to LLM for tag/summary generation.
// Gemma3 has 131k token context. Using 20k chars to match embedding limit,
// which is plenty for generating good tags and summaries.
const MaxContentForEnrichment = 20000

// EnrichDocument generates tags and summary for a document.
// Note: Runs sequentially because DMR can only handle one LLM request at a time.
func (c *Client) EnrichDocument(ctx context.Context, title, content string) (*EnrichmentResult, error) {
	// Truncate content if needed
	if len(content) > MaxContentForEnrichment {
		content = content[:MaxContentForEnrichment]
	}

	result := &EnrichmentResult{}

	// Generate search tags optimized for RAG retrieval
	tagsPrompt := fmt.Sprintf(`You are helping build a RAG (Retrieval-Augmented Generation) system for technical documentation search.

CONTEXT: We use hybrid search combining:
- BM25 (keyword matching) - finds exact term matches
- Vector search (semantic similarity) - finds conceptually related content

YOUR TASK: Generate 10-15 search terms that will help users find this document.

REQUIREMENTS:
1. Include SYNONYMS for key concepts (e.g., if doc mentions "function", add "method", "procedure")
2. Include RELATED CONCEPTS not explicitly in the text (e.g., if doc is about "HTTP servers", add "REST API", "web service")
3. Include COMMON MISSPELLINGS or alternative phrasings users might search
4. Include both TECHNICAL TERMS and PLAIN ENGLISH equivalents
5. Focus on terms a developer would actually type into a search box

DOCUMENT:
Title: %s

Content:
%s

OUTPUT FORMAT: Return ONLY comma-separated terms, no explanations, no numbering, no quotes.
Example: term1, term2, term3`, title, content)

	slog.Debug("generating tags", "title", title)
	tagsResp, err := c.Complete(ctx, tagsPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tags: %w", err)
	}

	// Parse tags
	for _, tag := range strings.Split(tagsResp, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result.Tags = append(result.Tags, tag)
		}
	}

	// Generate summary optimized for hybrid search
	summaryPrompt := fmt.Sprintf(`You are helping build a RAG (Retrieval-Augmented Generation) system for technical documentation search.

CONTEXT: This summary will be:
1. Indexed for BM25 keyword search - so include important technical terms
2. Embedded as a vector for semantic search - so capture the conceptual meaning
3. Shown to users in search results - so be clear and informative

YOUR TASK: Write a comprehensive summary (3-5 paragraphs) that maximizes searchability.

REQUIREMENTS:
1. FIRST PARAGRAPH: What is this document about? What problem does it solve?
2. SECOND PARAGRAPH: Key concepts, APIs, functions, or components mentioned
3. THIRD PARAGRAPH: Step-by-step procedures or workflows (if any)
4. FOURTH PARAGRAPH: Prerequisites, requirements, or related topics
5. Use SPECIFIC TECHNICAL TERMS that users would search for
6. Include ALTERNATIVE PHRASINGS for key concepts
7. Mention the TARGET AUDIENCE (beginners, advanced, etc.)

DOCUMENT:
Title: %s

Content:
%s

OUTPUT FORMAT: Return ONLY the summary paragraphs. No headers, no bullet points, no preamble like "This document...". Start directly with the content.`, title, content)

	slog.Debug("generating summary", "title", title)
	summaryResp, err := c.Complete(ctx, summaryPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	result.Summary = summaryResp

	return result, nil
}
