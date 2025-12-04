#!/bin/bash
set -e

echo "=== BAM-RAG Setup ==="
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Please install Docker Desktop 4.40+"
    exit 1
fi

echo "✓ Docker found"

# Check if Model Runner is available
if ! docker model ls &> /dev/null; then
    echo "❌ Docker Model Runner not enabled."
    echo "   Go to Docker Desktop → Settings → Features in development → Enable Model Runner"
    exit 1
fi

echo "✓ Docker Model Runner enabled"

# Pull models
echo ""
echo "Pulling AI models (this may take a few minutes)..."
echo ""

echo "→ Pulling Gemma3 (LLM for tags/summaries)..."
docker model pull ai/gemma3

echo "→ Pulling qwen3-embedding (vector embeddings)..."
docker model pull ai/qwen3-embedding

echo ""
echo "✓ Models ready"

# Start infrastructure
echo ""
echo "Starting infrastructure..."
docker compose up -d

# Wait for services
echo ""
echo "Waiting for services to be healthy..."
sleep 5

docker compose ps

# Detect socket path
SOCKET_PATH=""
if [ -S "$HOME/.docker/run/docker.sock" ]; then
    SOCKET_PATH="$HOME/.docker/run/docker.sock"
elif [ -S "/var/run/docker.sock" ]; then
    SOCKET_PATH="/var/run/docker.sock"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo ""
echo "1. Update config/config.yaml with your Docker socket path:"
if [ -n "$SOCKET_PATH" ]; then
    echo "   Detected: $SOCKET_PATH"
fi
echo ""
echo "2. Scrape some docs:"
echo "   go run ./cmd/bam-rag scrape --url https://go.dev/doc/tutorial/getting-started -v"
echo ""
echo "3. Search:"
echo "   go run ./cmd/bam-rag search \"getting started\""
echo ""
