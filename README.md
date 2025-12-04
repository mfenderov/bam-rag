# BAM-RAG

A toy RAG (Retrieval-Augmented Generation) system for learning and experimentation.

## What it does

Scrapes documentation → enriches with LLM → indexes with hybrid search → serves via MCP

```
[Scraper] → [S3] → [Ingestion] → [Elasticsearch]
                        ↓
              [LLM: tags/summary]
              [Embeddings: vectors]
```

## Quick Start

```bash
# Start infrastructure
docker compose up -d

# Scrape and index docs
go run ./cmd/bam-rag scrape --url https://go.dev/doc/tutorial/getting-started

# Search
go run ./cmd/bam-rag search "error handling"

# Start MCP server (for Claude Desktop)
go run ./cmd/bam-rag serve
```

## Stack

- **Go** - single binary, fast
- **Elasticsearch** - hybrid search (BM25 + vectors with RRF)
- **MinIO** - S3-compatible storage between scraper and indexer
- **Docker Model Runner** - local LLM (Gemma3) and embeddings (qwen3)

## Architecture

Event-driven with Go channels. Scraper writes to S3, sends event, ingestion worker processes independently. No orchestrator - pure choreography.

## License

MIT
