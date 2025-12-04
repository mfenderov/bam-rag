package pipeline

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/mfenderov/bam-rag/internal/embeddings"
	"github.com/mfenderov/bam-rag/internal/llm"
	"github.com/mfenderov/bam-rag/internal/markdown"
	"github.com/mfenderov/bam-rag/internal/processor"
	"github.com/mfenderov/bam-rag/internal/scraper"
	"github.com/mfenderov/bam-rag/pkg/models"
)

// ScraperConfig holds scraper-specific configuration.
type ScraperConfig struct {
	Delay            time.Duration
	MaxDepth         int
	FollowLinks      bool
	UserAgent        string
	TryMarkdownFirst bool
}

// EmbeddingsConfig holds embeddings-specific configuration.
type EmbeddingsConfig struct {
	Enabled    bool
	SocketPath string
	Model      string
}

// LLMConfig holds LLM enrichment configuration.
type LLMConfig struct {
	Enabled    bool
	SocketPath string
	Model      string
}

// Config holds pipeline configuration.
type Config struct {
	ESAddresses      []string
	ESIndex          string
	ESUsername       string
	ESPassword       string
	ScraperConfig    ScraperConfig
	EmbeddingsConfig EmbeddingsConfig
	LLMConfig        LLMConfig
}

// Result holds pipeline execution results.
type Result struct {
	PagesScraped int
	DocsIndexed  int
	Duration     time.Duration
	Errors       []error
}

// Pipeline orchestrates the scraping, processing, and indexing flow.
type Pipeline struct {
	config      Config
	esClient    *elasticsearch.Client
	scraper     *scraper.Scraper
	processor   *processor.Processor
	embedClient *embeddings.Client // nil if embeddings disabled
	llmClient   *llm.Client        // nil if LLM enrichment disabled
}

// New creates a new Pipeline with the given configuration.
func New(config Config) (*Pipeline, error) {
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: config.ESAddresses,
		Index:     config.ESIndex,
		Username:  config.ESUsername,
		Password:  config.ESPassword,
	})
	if err != nil {
		return nil, err
	}

	scraperInstance := scraper.New(scraper.Config{
		Delay:            config.ScraperConfig.Delay,
		MaxDepth:         config.ScraperConfig.MaxDepth,
		FollowLinks:      config.ScraperConfig.FollowLinks,
		UserAgent:        config.ScraperConfig.UserAgent,
		TryMarkdownFirst: config.ScraperConfig.TryMarkdownFirst,
	})

	// Optionally create embeddings client
	var embedClient *embeddings.Client
	if config.EmbeddingsConfig.Enabled {
		embedClient, err = embeddings.New(embeddings.Config{
			SocketPath: config.EmbeddingsConfig.SocketPath,
			Model:      config.EmbeddingsConfig.Model,
		})
		if err != nil {
			return nil, err
		}
		slog.Info("embeddings enabled", "model", config.EmbeddingsConfig.Model)
	}

	// Optionally create LLM client for enrichment
	var llmClient *llm.Client
	if config.LLMConfig.Enabled {
		llmClient, err = llm.New(llm.Config{
			SocketPath: config.LLMConfig.SocketPath,
			Model:      config.LLMConfig.Model,
		})
		if err != nil {
			return nil, err
		}
		slog.Info("LLM enrichment enabled", "model", config.LLMConfig.Model)
	}

	return &Pipeline{
		config:      config,
		esClient:    esClient,
		scraper:     scraperInstance,
		processor:   processor.New(),
		embedClient: embedClient,
		llmClient:   llmClient,
	}, nil
}

// Run executes the full pipeline for a given URL.
func (p *Pipeline) Run(ctx context.Context, startURL string) (*Result, error) {
	start := time.Now()
	result := &Result{}

	// Ensure index exists
	if err := p.esClient.CreateIndex(ctx); err != nil {
		return nil, err
	}

	// Scrape pages
	scrapedDocs, err := p.scraper.Scrape(ctx, startURL)
	if err != nil {
		result.Errors = append(result.Errors, err)
	}
	result.PagesScraped = len(scrapedDocs)

	// Process and index each document
	for _, scraped := range scrapedDocs {
		var mdContent string
		var title string

		// Check if content is already markdown
		isMarkdown := markdown.Detect(scraped.URL, scraped.ContentType, scraped.Content)

		if isMarkdown {
			// Content is already markdown - use directly
			mdContent = scraped.Content
			// For markdown, try to extract title from first H1
			title = extractMarkdownTitle(scraped.Content)
		} else {
			// Content is HTML - extract title and convert
			title = p.processor.ExtractTitle(scraped.Content)
			var err error
			mdContent, err = p.processor.Convert(scraped.Content)
			if err != nil {
				result.Errors = append(result.Errors, err)
				continue
			}
		}

		if title == "" {
			title = scraped.URL
		}

		// Create document with full markdown content
		doc := models.Document{
			ID:          models.GenerateDocumentID(scraped.URL),
			URL:         scraped.URL,
			Title:       title,
			Content:     mdContent,
			ContentType: scraped.ContentType,
			ScrapedAt:   scraped.ScrapedAt,
		}

		// Generate tags and summary using LLM if enabled
		// Note: Sequential execution is faster than parallel due to DMR GPU sharing
		if p.llmClient != nil {
			enrichment, err := p.llmClient.EnrichDocument(ctx, title, mdContent)
			if err != nil {
				slog.Warn("failed to enrich document", "url", scraped.URL, "error", err)
				// Continue without enrichment - basic BM25 will still work
			} else {
				doc.Tags = enrichment.Tags
				doc.Summary = enrichment.Summary
				slog.Debug("document enriched", "url", scraped.URL, "tags", len(doc.Tags))
			}
		}

		// Generate embedding of full content (qwen3-embedding supports ~24k chars)
		if p.embedClient != nil {
			embedding, err := p.embedClient.Embed(ctx, mdContent)
			if err != nil {
				slog.Warn("failed to generate embedding", "url", scraped.URL, "error", err)
			} else {
				doc.Embedding = embedding
			}
		}

		// Index the full document
		if err := p.esClient.IndexDocument(ctx, doc); err != nil {
			result.Errors = append(result.Errors, err)
		} else {
			result.DocsIndexed++
		}
	}

	// Refresh index to make documents searchable immediately
	p.esClient.Refresh(ctx)

	result.Duration = time.Since(start)
	return result, nil
}

// Search queries the indexed documents.
func (p *Pipeline) Search(ctx context.Context, query string, limit int) ([]models.Document, error) {
	return p.esClient.Search(ctx, query, limit)
}

// DeleteIndex removes the index (for testing/cleanup).
func (p *Pipeline) DeleteIndex(ctx context.Context) error {
	return p.esClient.DeleteIndex(ctx)
}

// extractMarkdownTitle extracts the first H1 heading from markdown content.
func extractMarkdownTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}
