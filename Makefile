.PHONY: help build test docker-up docker-down mcp-setup mcp-run clean lint deps

.DEFAULT_GOAL := help

GOCMD=go
BINARY=bam-rag
IMAGE=bam-rag:latest

## help: Show this help message
help:
	@echo "BAM-RAG - Documentation RAG System with MCP"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Test:"
	@echo "  build        Build the bam-rag binary"
	@echo "  test         Run all tests"
	@echo "  clean        Remove build artifacts"
	@echo ""
	@echo "Docker MCP Gateway:"
	@echo "  mcp-setup    Build image, start ES, register with gateway"
	@echo "  mcp-run      Run Docker MCP Gateway"
	@echo "  docker-up    Start Elasticsearch only"
	@echo "  docker-down  Stop Elasticsearch"
	@echo ""
	@echo "CLI Usage (after build):"
	@echo "  ./bam-rag scrape --url <url>    Scrape and index a URL"
	@echo "  ./bam-rag search \"<query>\"      Search indexed docs"
	@echo "  ./bam-rag serve                 Run MCP server (stdio)"

## build: Build the bam-rag binary
build:
	$(GOCMD) build -o $(BINARY) ./cmd/bam-rag

## test: Run all tests
test:
	$(GOCMD) test -v ./...

## docker-up: Start Elasticsearch
docker-up:
	docker-compose up -d
	@echo "Waiting for Elasticsearch..."
	@until curl -s http://localhost:9200/_cluster/health > /dev/null 2>&1; do sleep 2; done
	@echo "✅ Elasticsearch ready at http://localhost:9200"

## docker-down: Stop Elasticsearch
docker-down:
	docker-compose down

## docker-build: Build Docker image
docker-build:
	docker build -t $(IMAGE) .

## mcp-setup: Full setup for Docker MCP Gateway
mcp-setup: docker-build docker-up
	@echo ""
	@echo "🚀 Setting up bam-rag for Docker MCP Gateway..."
	docker mcp server enable bam-rag \
		--image $(IMAGE) \
		--network bam-rag-network \
		--env BAMRAG_ELASTICSEARCH_ADDRESSES=http://bam-rag-es:9200
	@echo ""
	@echo "✅ Done! Now run: make mcp-run"

## mcp-run: Run Docker MCP Gateway
mcp-run:
	docker mcp gateway run

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html

## lint: Run linter
lint:
	golangci-lint run ./...

## deps: Download and tidy dependencies
deps:
	$(GOCMD) mod download
	$(GOCMD) mod tidy
