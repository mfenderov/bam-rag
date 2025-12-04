package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/mfenderov/bam-rag/internal/markdown"
	"github.com/mfenderov/bam-rag/internal/storage"
	"github.com/mfenderov/bam-rag/pkg/models"
)

// Config holds scraper configuration.
type Config struct {
	Delay            time.Duration
	MaxDepth         int
	FollowLinks      bool
	UserAgent        string
	Timeout          time.Duration
	TryMarkdownFirst bool // Try to fetch markdown version of pages
}

// Scraper fetches web pages and returns their content.
type Scraper struct {
	config     Config
	httpClient *http.Client
}

// New creates a new Scraper with the given configuration.
func New(config Config) *Scraper {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "BAM-RAG/1.0"
	}
	return &Scraper{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Scrape fetches the given URL and optionally follows links.
// Returns a slice of documents containing the scraped content.
// The context can be used to cancel the scraping operation.
func (s *Scraper) Scrape(ctx context.Context, startURL string) ([]models.Document, error) {
	var docs []models.Document
	var mu sync.Mutex
	var cancelled bool

	slog.Debug("starting scrape", "url", startURL, "max_depth", s.config.MaxDepth)

	// Parse the start URL to get allowed domain
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		slog.Error("failed to parse URL", "url", startURL, "error", err)
		return nil, err
	}

	c := colly.NewCollector(
		colly.MaxDepth(s.config.MaxDepth),
		colly.UserAgent(s.config.UserAgent),
	)

	// Set rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       s.config.Delay,
		Parallelism: 2,
	})

	// Set timeout
	c.SetRequestTimeout(s.config.Timeout)

	// Check for cancellation before each request
	c.OnRequest(func(r *colly.Request) {
		if ctx.Err() != nil {
			slog.Debug("scrape cancelled", "url", r.URL.String())
			r.Abort()
			cancelled = true
		}
	})

	// Handle responses
	c.OnResponse(func(r *colly.Response) {
		if r.StatusCode >= 400 {
			slog.Debug("skipping page with error status", "url", r.Request.URL.String(), "status", r.StatusCode)
			return
		}

		pageURL := r.Request.URL.String()
		content := string(r.Body)
		contentType := r.Headers.Get("Content-Type")

		slog.Debug("scraped page", "url", pageURL, "content_type", contentType, "size", len(content))

		// Try markdown variants if enabled
		if s.config.TryMarkdownFirst {
			if mdContent, mdContentType, ok := s.tryMarkdownVariants(ctx, pageURL); ok {
				slog.Debug("using markdown variant", "url", pageURL)
				content = mdContent
				contentType = mdContentType
			}
		}

		doc := models.Document{
			URL:         pageURL,
			Content:     content,
			ContentType: contentType,
			ScrapedAt:   time.Now(),
		}

		mu.Lock()
		docs = append(docs, doc)
		mu.Unlock()
	})

	// Follow links if enabled
	if s.config.FollowLinks {
		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			link := e.Attr("href")
			absoluteURL := e.Request.AbsoluteURL(link)

			// Only follow links within the same domain
			linkURL, err := url.Parse(absoluteURL)
			if err != nil {
				return
			}
			if linkURL.Host == parsedURL.Host {
				e.Request.Visit(absoluteURL)
			}
		})
	}

	// Start scraping
	err = c.Visit(startURL)
	if err != nil {
		slog.Debug("visit error (continuing)", "url", startURL, "error", err)
		return docs, nil
	}

	// Wait for all requests to finish
	c.Wait()

	if cancelled {
		slog.Info("scrape cancelled by context", "pages_scraped", len(docs))
		return docs, ctx.Err()
	}

	slog.Debug("scrape complete", "url", startURL, "pages", len(docs))
	return docs, nil
}

// tryMarkdownVariants attempts to fetch markdown versions of the URL.
// Returns the content, content-type, and success flag.
func (s *Scraper) tryMarkdownVariants(ctx context.Context, pageURL string) (string, string, bool) {
	variants := markdown.MarkdownURLVariants(pageURL)

	for _, variantURL := range variants {
		if ctx.Err() != nil {
			return "", "", false
		}
		if content, contentType, ok := s.tryFetchMarkdown(ctx, variantURL); ok {
			return content, contentType, true
		}
	}

	return "", "", false
}

// tryFetchMarkdown attempts to fetch a single markdown URL.
func (s *Scraper) tryFetchMarkdown(ctx context.Context, url string) (string, string, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("User-Agent", s.config.UserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", false
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")

	if markdown.Detect(url, contentType, content) {
		return content, contentType, true
	}

	return "", "", false
}

// ScrapeResult holds the result of a ScrapeToS3 operation.
type ScrapeResult struct {
	Prefix    string // S3 prefix where files were written
	PageCount int    // Number of pages scraped
	SourceURL string // Original URL that was scraped
}

// ScrapeToS3 scrapes the given URL and writes results to S3.
// Returns the S3 prefix where the scrape was stored.
func (s *Scraper) ScrapeToS3(ctx context.Context, startURL string, storageClient *storage.Client) (*ScrapeResult, error) {
	// Parse the start URL to get the host for the prefix
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Generate unique prefix: scrapes/{host}/{timestamp}-{shortid}
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	shortID := models.GenerateDocumentID(fmt.Sprintf("%s-%d", startURL, time.Now().UnixNano()))[:8]
	prefix := fmt.Sprintf("scrapes/%s/%s-%s", parsedURL.Host, timestamp, shortID)

	slog.Info("starting scrape to S3", "url", startURL, "prefix", prefix)

	// Scrape pages using existing method
	docs, err := s.Scrape(ctx, startURL)
	if err != nil && len(docs) == 0 {
		return nil, fmt.Errorf("scrape failed: %w", err)
	}

	// Write each page to S3
	var pageURLs []string
	for _, doc := range docs {
		// Generate filename from URL hash
		filename := models.GenerateDocumentID(doc.URL) + ".md"

		// Get markdown content (already markdown or needs conversion)
		mdContent := doc.Content
		if !markdown.Detect(doc.URL, doc.ContentType, doc.Content) {
			// Content is HTML - for now just store as-is
			// The ingestion engine will handle conversion
			slog.Debug("storing HTML content", "url", doc.URL)
		}

		if err := storageClient.PutMarkdown(ctx, prefix, filename, mdContent); err != nil {
			slog.Error("failed to write to S3", "url", doc.URL, "error", err)
			continue
		}

		pageURLs = append(pageURLs, doc.URL)
		slog.Debug("wrote page to S3", "url", doc.URL, "filename", filename)
	}

	// Write metadata
	meta := storage.ScrapeMetadata{
		SourceURL: startURL,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		PageCount: len(pageURLs),
		Pages:     pageURLs,
	}
	if err := storageClient.PutMetadata(ctx, prefix, meta); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	slog.Info("scrape to S3 complete", "url", startURL, "prefix", prefix, "pages", len(pageURLs))

	return &ScrapeResult{
		Prefix:    prefix,
		PageCount: len(pageURLs),
		SourceURL: startURL,
	}, nil
}
