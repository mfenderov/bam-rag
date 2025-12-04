# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /bam-rag ./cmd/bam-rag

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /bam-rag /usr/local/bin/bam-rag

# Copy default config
COPY config/config.yaml /etc/bam-rag/config.yaml

# Set working directory
WORKDIR /app

# Default command runs MCP server
ENTRYPOINT ["/usr/local/bin/bam-rag"]
CMD ["serve"]
