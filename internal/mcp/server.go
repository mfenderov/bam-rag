package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mfenderov/bam-rag/internal/elasticsearch"
	"github.com/mfenderov/bam-rag/pkg/models"
)

// Config holds MCP server configuration.
type Config struct {
	Name        string
	Version     string
	ESAddresses []string
	ESIndex     string
	ESUsername  string
	ESPassword  string
}

// Server wraps the MCP server with Elasticsearch integration.
type Server struct {
	mcpServer *server.MCPServer
	esClient  *elasticsearch.Client
}

// NewServer creates a new MCP server with search tools.
func NewServer(config Config) (*Server, error) {
	esClient, err := elasticsearch.New(elasticsearch.Config{
		Addresses: config.ESAddresses,
		Index:     config.ESIndex,
		Username:  config.ESUsername,
		Password:  config.ESPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	mcpServer := server.NewMCPServer(
		config.Name,
		config.Version,
		server.WithToolCapabilities(true),
	)

	s := &Server{
		mcpServer: mcpServer,
		esClient:  esClient,
	}

	// Register search_documents tool
	searchTool := mcp.NewTool("search_documents",
		mcp.WithDescription("Search indexed documentation pages by query. Returns full page content in markdown format."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query string"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 10)"),
		),
	)
	mcpServer.AddTool(searchTool, s.searchHandler)

	// Register get_document tool
	getDocTool := mcp.NewTool("get_document",
		mcp.WithDescription("Get a specific documentation page by ID"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Document ID to retrieve"),
		),
	)
	mcpServer.AddTool(getDocTool, s.getDocumentHandler)

	return s, nil
}

// searchHandler handles the search_documents tool call.
func (s *Server) searchHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	limit := req.GetInt("limit", 10)

	docs, err := s.handleSearch(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	result, err := json.Marshal(docs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// getDocumentHandler handles the get_document tool call.
func (s *Server) getDocumentHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	doc, err := s.handleGetDocument(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get document failed: %v", err)), nil
	}

	if doc == nil {
		return mcp.NewToolResultError(fmt.Sprintf("document not found: %s", id)), nil
	}

	result, err := json.Marshal(doc)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal document: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleSearch searches for documents matching the query.
func (s *Server) handleSearch(ctx context.Context, query string, limit int) ([]models.Document, error) {
	return s.esClient.Search(ctx, query, limit)
}

// handleGetDocument retrieves a document by ID.
func (s *Server) handleGetDocument(ctx context.Context, id string) (*models.Document, error) {
	return s.esClient.GetDocument(ctx, id)
}

// ServeStdio starts the MCP server using stdio transport.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
