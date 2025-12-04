package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/spf13/cobra"
)

var (
	searchLimit  int
	searchFormat string
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search indexed documentation",
	Long: `Search the indexed documentation pages.

Examples:
  # Basic search
  bam-rag search "how to install"

  # Limit results
  bam-rag search "error handling" --limit 5

  # JSON output for scripting
  bam-rag search "modules" --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchFormat, "format", "text", "Output format: text or json")
}

func runSearch(cmd *cobra.Command, args []string) error {
	// Setup context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	query := args[0]
	cfg := GetConfig()

	// Create ES client
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
		Index:     cfg.Elasticsearch.Index,
		Username:  cfg.Elasticsearch.Username,
		Password:  cfg.Elasticsearch.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}

	// Perform search
	docs, err := esClient.Search(ctx, query, searchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(docs) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Output results
	if searchFormat == "json" {
		output, err := json.MarshalIndent(docs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(output))
	} else {
		fmt.Printf("Found %d results:\n\n", len(docs))
		for i, doc := range docs {
			fmt.Printf("─── Result %d ───\n", i+1)
			fmt.Printf("Title:   %s\n", doc.Title)
			fmt.Printf("URL:     %s\n", doc.URL)
			fmt.Printf("ID:      %s\n", doc.ID)

			// Truncate content for display
			content := doc.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			fmt.Printf("Content:\n%s\n\n", content)
		}
	}

	return nil
}
