package cmd

import (
	"fmt"

	"github.com/mfenderov/bam-rag/internal/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the MCP server for document retrieval.

The server communicates via stdio and provides two tools:
  - search_documents: Search indexed chunks by query
  - get_chunk: Get a specific chunk by ID

Example:
  bam-rag serve`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// Build MCP config from loaded configuration
	mcpConfig := mcp.Config{
		Name:        cfg.MCP.Name,
		Version:     cfg.MCP.Version,
		ESAddresses: cfg.Elasticsearch.Addresses,
		ESIndex:     cfg.Elasticsearch.Index,
		ESUsername:  cfg.Elasticsearch.Username,
		ESPassword:  cfg.Elasticsearch.Password,
	}

	server, err := mcp.NewServer(mcpConfig)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Starting MCP server...")

	return server.ServeStdio()
}
