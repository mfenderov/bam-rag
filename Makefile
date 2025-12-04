.PHONY: help setup models infra build test clean lint deps scrape search serve

.DEFAULT_GOAL := help

GOCMD=go
BINARY=bam-rag
IMAGE=bam-rag:latest

## help: Show this help message
help:
	@echo "BAM-RAG - Documentation RAG System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Setup (run these first):"
	@echo "  setup        Full setup: pull models + start infrastructure"
	@echo "  models       Pull required AI models (Gemma3 + qwen3-embedding)"
	@echo "  infra        Start infrastructure (Elasticsearch + MinIO)"
	@echo ""
	@echo "Build & Test:"
	@echo "  build        Build the bam-rag binary"
	@echo "  test         Run all tests"
	@echo "  deps         Download dependencies"
	@echo "  clean        Remove build artifacts"
	@echo ""
	@echo "Run:"
	@echo "  scrape URL=<url>    Scrape and index a URL"
	@echo "  search Q=\"<query>\"  Search indexed docs"
	@echo "  serve               Start MCP server"
	@echo ""
	@echo "Infrastructure:"
	@echo "  infra-up     Start Elasticsearch + MinIO"
	@echo "  infra-down   Stop all containers"
	@echo "  infra-logs   Show container logs"

## setup: Full setup - pull models and start infrastructure
setup: check-docker models infra
	@echo ""
	@echo "✅ Setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Update config/config.yaml with your Docker socket path"
	@echo "  2. Run: make scrape URL=https://go.dev/doc/tutorial/getting-started"
	@echo "  3. Run: make search Q=\"getting started\""

## check-docker: Verify Docker and Model Runner are available
check-docker:
	@command -v docker >/dev/null 2>&1 || { echo "❌ Docker not found. Install Docker Desktop 4.40+"; exit 1; }
	@docker model ls >/dev/null 2>&1 || { echo "❌ Docker Model Runner not enabled. Enable in Docker Desktop → Settings → Features in development"; exit 1; }
	@echo "✓ Docker and Model Runner ready"

## models: Pull required AI models
models: check-docker
	@echo "Pulling AI models (this may take a few minutes)..."
	docker model pull ai/gemma3
	docker model pull ai/qwen3-embedding
	@echo "✓ Models ready"

## infra: Start infrastructure (alias for infra-up)
infra: infra-up

## infra-up: Start Elasticsearch and MinIO
infra-up:
	docker compose up -d
	@echo "Waiting for services..."
	@until curl -s http://localhost:9200/_cluster/health > /dev/null 2>&1; do sleep 2; done
	@echo "✓ Elasticsearch ready at http://localhost:9200"
	@echo "✓ MinIO ready at http://localhost:9002 (console: http://localhost:9003)"

## infra-down: Stop all containers
infra-down:
	docker compose down

## infra-logs: Show container logs
infra-logs:
	docker compose logs -f

## build: Build the bam-rag binary
build:
	$(GOCMD) build -o $(BINARY) ./cmd/bam-rag

## test: Run all tests
test:
	$(GOCMD) test -v ./...

## scrape: Scrape and index a URL (usage: make scrape URL=https://example.com)
scrape:
ifndef URL
	@echo "Usage: make scrape URL=https://example.com/docs"
	@exit 1
endif
	$(GOCMD) run ./cmd/bam-rag scrape --url $(URL) -v

## search: Search indexed docs (usage: make search Q="query")
search:
ifndef Q
	@echo "Usage: make search Q=\"your search query\""
	@exit 1
endif
	$(GOCMD) run ./cmd/bam-rag search "$(Q)"

## serve: Start MCP server
serve:
	$(GOCMD) run ./cmd/bam-rag serve

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
