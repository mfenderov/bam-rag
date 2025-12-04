package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/mfenderov/bam-rag/pkg/models"
)

// Config holds Elasticsearch client configuration.
type Config struct {
	Addresses []string
	Index     string
	Username  string
	Password  string
}

// Client wraps the Elasticsearch client with RAG-specific operations.
type Client struct {
	es    *elasticsearch.Client
	index string
}

// New creates a new Elasticsearch client.
func New(config Config) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}

	return &Client{
		es:    es,
		index: config.Index,
	}, nil
}

// Ping checks if Elasticsearch is available.
func (c *Client) Ping(ctx context.Context) bool {
	res, err := c.es.Ping(c.es.Ping.WithContext(ctx))
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return !res.IsError()
}

// indexMapping defines the ES index mapping for documents.
// Supports LLM-generated tags/summary and optional vector embeddings.
var indexMapping = `{
	"mappings": {
		"properties": {
			"id": { "type": "keyword" },
			"url": { "type": "keyword" },
			"title": { "type": "text" },
			"content": { "type": "text", "analyzer": "english" },
			"content_type": { "type": "keyword" },
			"scraped_at": { "type": "date" },
			"tags": { "type": "text", "analyzer": "english" },
			"summary": { "type": "text", "analyzer": "english" },
			"embedding": {
				"type": "dense_vector",
				"dims": 2560,
				"index": true,
				"similarity": "cosine"
			}
		}
	}
}`

// CreateIndex creates the index with proper mapping.
func (c *Client) CreateIndex(ctx context.Context) error {
	// Check if index exists
	res, err := c.es.Indices.Exists([]string{c.index}, c.es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to check index: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		// Index already exists
		return nil
	}

	// Create index
	res, err = c.es.Indices.Create(
		c.index,
		c.es.Indices.Create.WithContext(ctx),
		c.es.Indices.Create.WithBody(bytes.NewReader([]byte(indexMapping))),
	)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating index: %s", res.String())
	}

	return nil
}

// DeleteIndex removes the index (for testing/cleanup).
func (c *Client) DeleteIndex(ctx context.Context) error {
	res, err := c.es.Indices.Delete([]string{c.index}, c.es.Indices.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// IndexDocument indexes a single document.
func (c *Client) IndexDocument(ctx context.Context, doc models.Document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	res, err := c.es.Index(
		c.index,
		bytes.NewReader(data),
		c.es.Index.WithContext(ctx),
		c.es.Index.WithDocumentID(doc.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing document (status %d): %s", res.StatusCode, res.String())
	}

	return nil
}

// Refresh forces an index refresh (useful for testing).
func (c *Client) Refresh(ctx context.Context) error {
	res, err := c.es.Indices.Refresh(
		c.es.Indices.Refresh.WithContext(ctx),
		c.es.Indices.Refresh.WithIndex(c.index),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// searchResponse represents ES search response structure.
type searchResponse struct {
	Hits struct {
		Hits []struct {
			Source models.Document `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// Search performs a BM25 text search on document content, title, tags, and summary.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]models.Document, error) {
	searchQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query,
				"fields": []string{"content", "title", "tags^2", "summary"},
			},
		},
		"size": limit,
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index),
		c.es.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var sr searchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	docs := make([]models.Document, len(sr.Hits.Hits))
	for i, hit := range sr.Hits.Hits {
		docs[i] = hit.Source
	}

	return docs, nil
}

// getResponse represents ES get response structure.
type getResponse struct {
	Found  bool            `json:"found"`
	Source models.Document `json:"_source"`
}

// HybridSearch performs a combined BM25 + vector search.
// If queryEmbedding is nil, falls back to BM25 only.
func (c *Client) HybridSearch(ctx context.Context, query string, queryEmbedding []float32, limit int) ([]models.Document, error) {
	if queryEmbedding == nil {
		return c.Search(ctx, query, limit)
	}

	// Use reciprocal rank fusion (RRF) to combine BM25 and vector results
	searchQuery := map[string]interface{}{
		"retriever": map[string]interface{}{
			"rrf": map[string]interface{}{
				"retrievers": []map[string]interface{}{
					{
						"standard": map[string]interface{}{
							"query": map[string]interface{}{
								"multi_match": map[string]interface{}{
									"query":  query,
									"fields": []string{"content", "title"},
								},
							},
						},
					},
					{
						"knn": map[string]interface{}{
							"field":           "embedding",
							"query_vector":    queryEmbedding,
							"k":               limit,
							"num_candidates":  limit * 2,
						},
					},
				},
			},
		},
		"size": limit,
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index),
		c.es.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("hybrid search error: %s", res.String())
	}

	var sr searchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	docs := make([]models.Document, len(sr.Hits.Hits))
	for i, hit := range sr.Hits.Hits {
		docs[i] = hit.Source
	}

	return docs, nil
}

// GetDocument retrieves a document by ID.
func (c *Client) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	res, err := c.es.Get(
		c.index,
		id,
		c.es.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("get failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, nil
	}

	if res.IsError() {
		return nil, fmt.Errorf("get error: %s", res.String())
	}

	var gr getResponse
	if err := json.NewDecoder(res.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !gr.Found {
		return nil, nil
	}

	return &gr.Source, nil
}
