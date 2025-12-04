package events

import "time"

// ScrapeCompleteEvent is sent when scraper finishes writing to S3.
type ScrapeCompleteEvent struct {
	Bucket    string    // S3 bucket name (e.g., "bam-rag")
	Prefix    string    // S3 prefix (e.g., "scrapes/go.dev/2024-12-04T17-30-00-abc123")
	SourceURL string    // Original URL that was scraped
	PageCount int       // Number of pages scraped
	Timestamp time.Time // When the scrape completed
}

// IngestionCompleteEvent is sent when ingestion finishes indexing.
type IngestionCompleteEvent struct {
	Prefix      string        // S3 prefix that was ingested
	DocsIndexed int           // Number of documents indexed
	Duration    time.Duration // How long ingestion took
	Errors      []string      // Any errors encountered (non-fatal)
}
