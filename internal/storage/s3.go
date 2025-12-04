package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds S3/MinIO client configuration.
type Config struct {
	Endpoint        string // "localhost:9000" for MinIO
	Bucket          string // "bam-rag"
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

// Client wraps the MinIO/S3 client for bam-rag operations.
type Client struct {
	minioClient *minio.Client
	bucket      string
}

// New creates a new S3/MinIO client.
func New(config Config) (*Client, error) {
	if config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if config.Bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &Client{
		minioClient: minioClient,
		bucket:      config.Bucket,
	}, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.minioClient.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}
	if exists {
		return nil
	}

	err = c.minioClient.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	return nil
}

// ScrapeMetadata holds information about a scrape operation.
type ScrapeMetadata struct {
	SourceURL string   `json:"source_url"`
	Timestamp string   `json:"timestamp"`
	PageCount int      `json:"page_count"`
	Pages     []string `json:"pages"` // List of page URLs scraped
}

// PutMarkdown writes a markdown file to S3.
func (c *Client) PutMarkdown(ctx context.Context, prefix, filename, content string) error {
	objectName := path.Join(prefix, "pages", filename)
	reader := strings.NewReader(content)

	_, err := c.minioClient.PutObject(ctx, c.bucket, objectName, reader, int64(len(content)), minio.PutObjectOptions{
		ContentType: "text/markdown",
	})
	if err != nil {
		return fmt.Errorf("failed to put markdown: %w", err)
	}
	return nil
}

// PutMetadata writes the scrape metadata JSON to S3.
func (c *Client) PutMetadata(ctx context.Context, prefix string, meta ScrapeMetadata) error {
	objectName := path.Join(prefix, "metadata.json")

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	reader := bytes.NewReader(data)
	_, err = c.minioClient.PutObject(ctx, c.bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return fmt.Errorf("failed to put metadata: %w", err)
	}
	return nil
}

// ListMarkdownFiles returns all markdown files under a prefix.
func (c *Client) ListMarkdownFiles(ctx context.Context, prefix string) ([]string, error) {
	pagesPrefix := path.Join(prefix, "pages") + "/"
	var files []string

	objectCh := c.minioClient.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    pagesPrefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}
		if strings.HasSuffix(object.Key, ".md") {
			// Return just the filename, not the full path
			files = append(files, path.Base(object.Key))
		}
	}

	return files, nil
}

// GetMarkdown reads a markdown file from S3.
func (c *Client) GetMarkdown(ctx context.Context, prefix, filename string) (string, error) {
	objectName := path.Join(prefix, "pages", filename)

	object, err := c.minioClient.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get markdown: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return "", fmt.Errorf("failed to read markdown: %w", err)
	}

	return string(data), nil
}

// GetMetadata reads the scrape metadata from S3.
func (c *Client) GetMetadata(ctx context.Context, prefix string) (*ScrapeMetadata, error) {
	objectName := path.Join(prefix, "metadata.json")

	object, err := c.minioClient.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta ScrapeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &meta, nil
}

// Bucket returns the bucket name.
func (c *Client) Bucket() string {
	return c.bucket
}
