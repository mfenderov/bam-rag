package ingestion

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
	"github.com/mfenderov/bam-rag/internal/storage"
	"github.com/mfenderov/bam-rag/pkg/models"
)

// Config holds ingestion engine configuration.
type Config struct {
	ESAddresses []string
	ESIndex     string
	ESUsername  string
	ESPassword  string
}

// Result holds ingestion execution results.
type Result struct {
	Prefix      string
	DocsIndexed int
	Duration    time.Duration
	Errors      []string
}

// Engine reads scraped content from S3, enriches it, and indexes to Elasticsearch.
type Engine struct {
	storage     *storage.Client
	esClient    *elasticsearch.Client
	processor   *processor.Processor
	embedClient *embeddings.Client // nil if embeddings disabled
	llmClient   *llm.Client        // nil if LLM enrichment disabled
}

// New creates a new ingestion engine.
func New(
	storageClient *storage.Client,
	esClient *elasticsearch.Client,
	embedClient *embeddings.Client,
	llmClient *llm.Client,
) *Engine {
	return &Engine{
		storage:     storageClient,
		esClient:    esClient,
		processor:   processor.New(),
		embedClient: embedClient,
		llmClient:   llmClient,
	}
}

// Ingest processes all documents from an S3 prefix and indexes them.
func (e *Engine) Ingest(ctx context.Context, prefix string) (*Result, error) {
	start := time.Now()
	result := &Result{Prefix: prefix}

	slog.Info("starting ingestion", "prefix", prefix)

	// Ensure ES index exists
	if err := e.esClient.CreateIndex(ctx); err != nil {
		return nil, err
	}

	// Get metadata for URL mapping
	meta, err := e.storage.GetMetadata(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// Build URL -> filename mapping from metadata
	urlToFile := make(map[string]string)
	for _, pageURL := range meta.Pages {
		filename := models.GenerateDocumentID(pageURL) + ".md"
		urlToFile[filename] = pageURL
	}

	// List all markdown files
	files, err := e.storage.ListMarkdownFiles(ctx, prefix)
	if err != nil {
		return nil, err
	}

	slog.Info("found files to ingest", "count", len(files))

	// Process each file
	for _, filename := range files {
		if ctx.Err() != nil {
			result.Errors = append(result.Errors, "context cancelled")
			break
		}

		// Get the original URL from metadata
		pageURL, ok := urlToFile[filename]
		if !ok {
			slog.Warn("no URL found for file", "filename", filename)
			pageURL = filename // fallback
		}

		// Read content from S3
		content, err := e.storage.GetMarkdown(ctx, prefix, filename)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		// Process the content
		doc, err := e.processDocument(ctx, pageURL, content)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		// Index to Elasticsearch
		slog.Debug("indexing document", "id", doc.ID, "url", doc.URL, "tags", len(doc.Tags))
		if err := e.esClient.IndexDocument(ctx, *doc); err != nil {
			slog.Error("failed to index document", "id", doc.ID, "error", err)
			result.Errors = append(result.Errors, err.Error())
		} else {
			slog.Debug("document indexed successfully", "id", doc.ID)
			result.DocsIndexed++
		}
	}

	// Refresh index to make documents searchable immediately
	e.esClient.Refresh(ctx)

	result.Duration = time.Since(start)
	slog.Info("ingestion complete",
		"prefix", prefix,
		"docs_indexed", result.DocsIndexed,
		"duration", result.Duration,
		"errors", len(result.Errors))

	return result, nil
}

// processDocument converts content to markdown, enriches with LLM/embeddings.
func (e *Engine) processDocument(ctx context.Context, pageURL, content string) (*models.Document, error) {
	var mdContent string
	var title string

	// Check if content is already markdown
	isMarkdown := markdown.Detect(pageURL, "", content)

	if isMarkdown {
		mdContent = content
		title = extractMarkdownTitle(content)
	} else {
		// Content is HTML - extract title and convert
		title = e.processor.ExtractTitle(content)
		var err error
		mdContent, err = e.processor.Convert(content)
		if err != nil {
			return nil, err
		}
	}

	if title == "" {
		title = pageURL
	}

	// Create document
	doc := models.Document{
		ID:        models.GenerateDocumentID(pageURL),
		URL:       pageURL,
		Title:     title,
		Content:   mdContent,
		ScrapedAt: time.Now(),
	}

	// Generate tags and summary using LLM if enabled
	if e.llmClient != nil {
		enrichment, err := e.llmClient.EnrichDocument(ctx, title, mdContent)
		if err != nil {
			slog.Warn("failed to enrich document", "url", pageURL, "error", err)
		} else {
			doc.Tags = enrichment.Tags
			doc.Summary = enrichment.Summary
			slog.Debug("document enriched", "url", pageURL, "tags", len(doc.Tags))
		}
	}

	// Generate embedding if enabled
	if e.embedClient != nil {
		embedding, err := e.embedClient.Embed(ctx, mdContent)
		if err != nil {
			slog.Warn("failed to generate embedding", "url", pageURL, "error", err)
		} else {
			doc.Embedding = embedding
		}
	}

	return &doc, nil
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
