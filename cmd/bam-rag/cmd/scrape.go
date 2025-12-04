package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/mfenderov/bam-rag/internal/config"
	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/mfenderov/bam-rag/internal/embeddings"
	"github.com/mfenderov/bam-rag/internal/events"
	"github.com/mfenderov/bam-rag/internal/ingestion"
	"github.com/mfenderov/bam-rag/internal/llm"
	"github.com/mfenderov/bam-rag/internal/pipeline"
	"github.com/mfenderov/bam-rag/internal/scraper"
	"github.com/mfenderov/bam-rag/internal/storage"
	"github.com/spf13/cobra"
)

var (
	scrapeURL    string
	scrapeSource string
	noIngest     bool
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape and index documentation",
	Long: `Scrape documentation from configured sources or a specific URL.

Examples:
  # Scrape all configured sources (scrape + ingest)
  bam-rag scrape

  # Scrape a specific source by name
  bam-rag scrape --source example-docs

  # Scrape a specific URL directly
  bam-rag scrape --url https://example.com/docs

  # Scrape only (write to S3, no ingestion)
  bam-rag scrape --url https://example.com/docs --no-ingest`,
	RunE: runScrape,
}

func init() {
	rootCmd.AddCommand(scrapeCmd)

	scrapeCmd.Flags().StringVar(&scrapeURL, "url", "", "URL to scrape directly")
	scrapeCmd.Flags().StringVar(&scrapeSource, "source", "", "Source name from config to scrape")
	scrapeCmd.Flags().BoolVar(&noIngest, "no-ingest", false, "Scrape to S3 only, skip ingestion")
}

func runScrape(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := GetConfig()
	slog.Debug("scrape command starting", "verbose", verbose, "no_ingest", noIngest)

	// Determine what to scrape
	var urls []string

	if scrapeURL != "" {
		urls = append(urls, scrapeURL)
	} else {
		if len(cfg.Sources) == 0 {
			return fmt.Errorf("no sources configured and no --url provided")
		}

		for _, source := range cfg.Sources {
			if scrapeSource != "" && source.Name != scrapeSource {
				continue
			}
			if source.URL != "" {
				urls = append(urls, source.URL)
			}
		}

		if len(urls) == 0 {
			if scrapeSource != "" {
				return fmt.Errorf("source %q not found in config", scrapeSource)
			}
			return fmt.Errorf("no valid sources found in config")
		}
	}

	// Use event-driven flow when S3 storage is configured
	if cfg.Storage.Endpoint != "" {
		return runEventDrivenScrape(ctx, &cfg, urls)
	}

	// Fallback to legacy pipeline for backward compatibility
	return runLegacyPipeline(ctx, &cfg, urls)
}

// runEventDrivenScrape uses the new event-driven architecture
func runEventDrivenScrape(ctx context.Context, cfg *config.Config, urls []string) error {
	// Create storage client
	storageClient, err := storage.New(storage.Config{
		Endpoint:        cfg.Storage.Endpoint,
		Bucket:          cfg.Storage.Bucket,
		AccessKeyID:     cfg.Storage.AccessKeyID,
		SecretAccessKey: cfg.Storage.SecretAccessKey,
		UseSSL:          cfg.Storage.UseSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	// Ensure bucket exists
	if err := storageClient.EnsureBucket(ctx); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

	// Create scraper
	scraperInstance := scraper.New(scraper.Config{
		Delay:            cfg.Scraper.Delay,
		MaxDepth:         cfg.Scraper.MaxDepth,
		FollowLinks:      cfg.Scraper.FollowLinks,
		Timeout:          cfg.Scraper.Timeout,
		UserAgent:        cfg.Scraper.UserAgent,
		TryMarkdownFirst: cfg.Scraper.TryMarkdownFirst,
	})

	if noIngest {
		// Scrape only mode - just write to S3
		return runScrapeOnly(ctx, scraperInstance, storageClient, urls)
	}

	// Full event-driven flow with ingestion
	return runScrapeWithIngest(ctx, cfg, scraperInstance, storageClient, urls)
}

// runScrapeOnly writes scraped content to S3 without ingestion
func runScrapeOnly(ctx context.Context, s *scraper.Scraper, storageClient *storage.Client, urls []string) error {
	totalPages := 0

	for _, url := range urls {
		fmt.Printf("Scraping to S3: %s\n", url)

		result, err := s.ScrapeToS3(ctx, url, storageClient)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		totalPages += result.PageCount
		fmt.Printf("  Pages: %d, Prefix: %s\n", result.PageCount, result.Prefix)
	}

	fmt.Printf("\nTotal: %d pages written to S3\n", totalPages)
	fmt.Println("Run 'bam-rag ingest --prefix <prefix>' to index these documents")
	return nil
}

// runScrapeWithIngest uses channels to coordinate scraping and ingestion
func runScrapeWithIngest(ctx context.Context, cfg *config.Config, s *scraper.Scraper, storageClient *storage.Client, urls []string) error {
	// Create ES client
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
		Index:     cfg.Elasticsearch.Index,
		Username:  cfg.Elasticsearch.Username,
		Password:  cfg.Elasticsearch.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Create optional embeddings client
	var embedClient *embeddings.Client
	if cfg.Embeddings.Enabled {
		embedClient, err = embeddings.New(embeddings.Config{
			SocketPath: cfg.Embeddings.SocketPath,
			Model:      cfg.Embeddings.Model,
		})
		if err != nil {
			return fmt.Errorf("failed to create embeddings client: %w", err)
		}
		slog.Info("embeddings enabled", "model", cfg.Embeddings.Model)
	}

	// Create optional LLM client
	var llmClient *llm.Client
	if cfg.LLM.Enabled {
		llmClient, err = llm.New(llm.Config{
			SocketPath: cfg.LLM.SocketPath,
			Model:      cfg.LLM.Model,
		})
		if err != nil {
			return fmt.Errorf("failed to create LLM client: %w", err)
		}
		slog.Info("LLM enrichment enabled", "model", cfg.LLM.Model)
	}

	// Create ingestion engine
	engine := ingestion.New(storageClient, esClient, embedClient, llmClient)

	// Event channel for scrape completion
	scrapeEvents := make(chan events.ScrapeCompleteEvent)
	done := make(chan struct{})

	// Track results
	var totalDocsIndexed int
	var totalDuration time.Duration

	// Start ingestion worker (consumer)
	go func() {
		defer close(done)
		for event := range scrapeEvents {
			fmt.Printf("Ingesting: %s (%d pages)\n", event.Prefix, event.PageCount)

			result, err := engine.Ingest(ctx, event.Prefix)
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
				continue
			}

			totalDocsIndexed += result.DocsIndexed
			totalDuration += result.Duration

			fmt.Printf("  Docs indexed: %d, Duration: %v\n", result.DocsIndexed, result.Duration)
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					fmt.Printf("  Warning: %s\n", e)
				}
			}
		}
	}()

	// Scrape URLs (producer)
	totalPages := 0
	for _, url := range urls {
		fmt.Printf("Scraping: %s\n", url)

		result, err := s.ScrapeToS3(ctx, url, storageClient)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		totalPages += result.PageCount
		fmt.Printf("  Pages: %d, Prefix: %s\n", result.PageCount, result.Prefix)

		// Send event to ingestion worker
		scrapeEvents <- events.ScrapeCompleteEvent{
			Bucket:    storageClient.Bucket(),
			Prefix:    result.Prefix,
			SourceURL: result.SourceURL,
			PageCount: result.PageCount,
			Timestamp: time.Now(),
		}
	}

	// Close channel and wait for ingestion to complete
	close(scrapeEvents)
	<-done

	fmt.Printf("\nTotal: %d pages scraped, %d docs indexed in %v\n",
		totalPages, totalDocsIndexed, totalDuration)

	return nil
}

// runLegacyPipeline uses the original direct pipeline for backward compatibility
func runLegacyPipeline(ctx context.Context, cfg *config.Config, urls []string) error {
	pipelineConfig := pipeline.Config{
		ESAddresses: cfg.Elasticsearch.Addresses,
		ESIndex:     cfg.Elasticsearch.Index,
		ESUsername:  cfg.Elasticsearch.Username,
		ESPassword:  cfg.Elasticsearch.Password,
		ScraperConfig: pipeline.ScraperConfig{
			Delay:            cfg.Scraper.Delay,
			MaxDepth:         cfg.Scraper.MaxDepth,
			FollowLinks:      cfg.Scraper.FollowLinks,
			UserAgent:        cfg.Scraper.UserAgent,
			TryMarkdownFirst: cfg.Scraper.TryMarkdownFirst,
		},
		EmbeddingsConfig: pipeline.EmbeddingsConfig{
			Enabled:    cfg.Embeddings.Enabled,
			SocketPath: cfg.Embeddings.SocketPath,
			Model:      cfg.Embeddings.Model,
		},
		LLMConfig: pipeline.LLMConfig{
			Enabled:    cfg.LLM.Enabled,
			SocketPath: cfg.LLM.SocketPath,
			Model:      cfg.LLM.Model,
		},
	}

	p, err := pipeline.New(pipelineConfig)
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	totalPages := 0
	totalDocs := 0
	var totalDuration time.Duration

	for _, url := range urls {
		fmt.Printf("Scraping: %s\n", url)

		result, err := p.Run(ctx, url)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		totalPages += result.PagesScraped
		totalDocs += result.DocsIndexed
		totalDuration += result.Duration

		fmt.Printf("  Pages: %d, Docs indexed: %d, Duration: %v\n",
			result.PagesScraped, result.DocsIndexed, result.Duration)

		if len(result.Errors) > 0 {
			for _, e := range result.Errors {
				fmt.Printf("  Warning: %v\n", e)
			}
		}
	}

	fmt.Printf("\nTotal: %d pages, %d docs indexed in %v\n",
		totalPages, totalDocs, totalDuration)

	return nil
}
