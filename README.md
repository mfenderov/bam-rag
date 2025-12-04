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

## Prerequisites

- **Docker Desktop 4.40+** with Model Runner enabled
- **Go 1.21+**

### Enable Docker Model Runner

1. Open Docker Desktop → Settings → Features in development
2. Enable "Docker Model Runner"
3. Restart Docker Desktop

### Pull Required Models

```bash
# LLM for generating tags and summaries
docker model pull ai/gemma3

# Embedding model for vector search
docker model pull ai/qwen3-embedding
```

## Quick Start

```bash
# 1. Start infrastructure (Elasticsearch + MinIO)
docker compose up -d

# 2. Wait for services to be healthy
docker compose ps

# 3. Update config with your Docker socket path
#    Edit config/config.yaml and set socket_path to your Docker socket
#    Usually: ~/.docker/run/docker.sock (Mac) or /var/run/docker.sock (Linux)

# 4. Scrape and index docs
go run ./cmd/bam-rag scrape --url https://go.dev/doc/tutorial/getting-started -v

# 5. Search
go run ./cmd/bam-rag search "error handling"

# 6. Start MCP server (for Claude Desktop)
go run ./cmd/bam-rag serve
```

## One-liner Setup

```bash
# Run setup script (pulls models + starts infra)
./scripts/setup.sh
```

Or manually:
```bash
docker model pull ai/gemma3 && docker model pull ai/qwen3-embedding && docker compose up -d
```

## Stack

- **Go** - single binary, fast
- **Elasticsearch** - hybrid search (BM25 + vectors with RRF)
- **MinIO** - S3-compatible storage between scraper and indexer
- **Docker Model Runner** - local LLM (Gemma3) and embeddings (qwen3)

## Architecture

Event-driven with Go channels. Scraper writes to S3, sends event, ingestion worker processes independently. No orchestrator - pure choreography.

## Configuration

Edit `config/config.yaml`:

```yaml
# Point to your Docker socket for Model Runner
embeddings:
  socket_path: /Users/YOUR_USER/.docker/run/docker.sock

llm:
  socket_path: /Users/YOUR_USER/.docker/run/docker.sock
```

## License

MIT
