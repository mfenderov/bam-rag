package config

import "time"

// Config holds all application configuration.
type Config struct {
	Elasticsearch Elasticsearch `mapstructure:"elasticsearch"`
	Embeddings    Embeddings    `mapstructure:"embeddings"`
	LLM           LLM           `mapstructure:"llm"`
	Scraper       Scraper       `mapstructure:"scraper"`
	Storage       Storage       `mapstructure:"storage"`
	MCP           MCP           `mapstructure:"mcp"`
	Sources       []Source      `mapstructure:"sources"`
}

// Elasticsearch holds ES connection configuration.
type Elasticsearch struct {
	Addresses []string `mapstructure:"addresses"`
	Index     string   `mapstructure:"index"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
}

// Embeddings holds embeddings generation configuration.
type Embeddings struct {
	Enabled    bool   `mapstructure:"enabled"`
	SocketPath string `mapstructure:"socket_path"`
	Model      string `mapstructure:"model"`
}

// LLM holds LLM enrichment configuration for tag/summary generation.
type LLM struct {
	Enabled    bool   `mapstructure:"enabled"`
	SocketPath string `mapstructure:"socket_path"`
	Model      string `mapstructure:"model"`
}

// Scraper holds web scraping configuration.
type Scraper struct {
	Delay            time.Duration `mapstructure:"delay"`
	MaxDepth         int           `mapstructure:"max_depth"`
	FollowLinks      bool          `mapstructure:"follow_links"`
	Timeout          time.Duration `mapstructure:"timeout"`
	UserAgent        string        `mapstructure:"user_agent"`
	TryMarkdownFirst bool          `mapstructure:"try_markdown_first"`
}

// Storage holds S3/MinIO storage configuration.
type Storage struct {
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
}

// MCP holds MCP server configuration.
type MCP struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// Source defines a documentation source to scrape.
type Source struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

// Defaults returns a Config with sensible default values.
func Defaults() Config {
	return Config{
		Elasticsearch: Elasticsearch{
			Addresses: []string{"http://localhost:9200"},
			Index:     "bam-rag-chunks",
		},
		Embeddings: Embeddings{
			Enabled:    false, // Disabled by default, requires DMR setup
			SocketPath: "",    // User must provide their Docker socket path
			Model:      "ai/embeddinggemma",
		},
		LLM: LLM{
			Enabled:    false, // Disabled by default, requires DMR setup
			SocketPath: "",    // User must provide their Docker socket path
			Model:      "ai/gemma3",
		},
		Scraper: Scraper{
			Delay:            1 * time.Second,
			MaxDepth:         3,
			FollowLinks:      true,
			Timeout:          30 * time.Second,
			UserAgent:        "bam-rag/1.0",
			TryMarkdownFirst: true, // Try markdown versions of pages first
		},
		Storage: Storage{
			Endpoint:        "localhost:9002",
			Bucket:          "bam-rag",
			AccessKeyID:     "minioadmin",
			SecretAccessKey: "minioadmin",
			UseSSL:          false,
		},
		MCP: MCP{
			Name:    "bam-rag",
			Version: "1.0.0",
		},
	}
}
