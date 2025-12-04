package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/mfenderov/bam-rag/internal/embeddings"
	"github.com/mfenderov/bam-rag/internal/ingestion"
	"github.com/mfenderov/bam-rag/internal/llm"
	"github.com/mfenderov/bam-rag/internal/storage"
	"github.com/spf13/cobra"
)

var ingestPrefix string

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest documents from S3 into Elasticsearch",
	Long: `Ingest previously scraped documents from S3 into Elasticsearch.

Use this command to re-run ingestion on existing scraped content,
or to index scrapes that were created with --no-ingest.

Examples:
  # Ingest a specific scrape by prefix
  bam-rag ingest --prefix scrapes/go.dev/2024-12-04T17-30-00-abc123`,
	RunE: runIngest,
}

func init() {
	rootCmd.AddCommand(ingestCmd)

	ingestCmd.Flags().StringVar(&ingestPrefix, "prefix", "", "S3 prefix to ingest (required)")
	ingestCmd.MarkFlagRequired("prefix")
}

func runIngest(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := GetConfig()
	slog.Debug("ingest command starting", "prefix", ingestPrefix)

	if cfg.Storage.Endpoint == "" {
		return fmt.Errorf("storage not configured - check config file")
	}

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

	fmt.Printf("Ingesting: %s\n", ingestPrefix)

	result, err := engine.Ingest(ctx, ingestPrefix)
	if err != nil {
		return fmt.Errorf("ingestion failed: %w", err)
	}

	fmt.Printf("\nIngestion complete:\n")
	fmt.Printf("  Docs indexed: %d\n", result.DocsIndexed)
	fmt.Printf("  Duration: %v\n", result.Duration)

	if len(result.Errors) > 0 {
		fmt.Printf("  Warnings: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	return nil
}
