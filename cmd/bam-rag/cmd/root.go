package cmd

import (
	"log/slog"
	"os"
	"strings"

	"github.com/mfenderov/bam-rag/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	cfg     config.Config
)

// GetConfig returns the loaded configuration.
func GetConfig() config.Config {
	return cfg
}

var rootCmd = &cobra.Command{
	Use:   "bam-rag",
	Short: "BAM-RAG: A documentation retrieval system",
	Long: `BAM-RAG scrapes documentation websites, converts HTML to Markdown,
chunks by headers, stores in Elasticsearch, and provides MCP tools for retrieval.

Commands:
  scrape  Scrape and index documentation from configured sources
  serve   Start the MCP server for document retrieval`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig, initLogger)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func initLogger() {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func initConfig() {
	// Start with defaults
	cfg = config.Defaults()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/bam-rag")
		viper.AddConfigPath(".")
	}

	// Environment variable overrides
	// BAMRAG_ELASTICSEARCH_ADDRESSES -> elasticsearch.addresses
	viper.SetEnvPrefix("BAMRAG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicitly bind nested env vars
	viper.BindEnv("elasticsearch.addresses", "BAMRAG_ELASTICSEARCH_ADDRESSES")
	viper.BindEnv("elasticsearch.index", "BAMRAG_ELASTICSEARCH_INDEX")
	viper.BindEnv("elasticsearch.username", "BAMRAG_ELASTICSEARCH_USERNAME")
	viper.BindEnv("elasticsearch.password", "BAMRAG_ELASTICSEARCH_PASSWORD")
	viper.BindEnv("embeddings.enabled", "BAMRAG_EMBEDDINGS_ENABLED")
	viper.BindEnv("embeddings.socket_path", "BAMRAG_EMBEDDINGS_SOCKET_PATH")
	viper.BindEnv("embeddings.model", "BAMRAG_EMBEDDINGS_MODEL")
	viper.BindEnv("llm.enabled", "BAMRAG_LLM_ENABLED")
	viper.BindEnv("llm.socket_path", "BAMRAG_LLM_SOCKET_PATH")
	viper.BindEnv("llm.model", "BAMRAG_LLM_MODEL")
	viper.BindEnv("scraper.delay", "BAMRAG_SCRAPER_DELAY")
	viper.BindEnv("scraper.max_depth", "BAMRAG_SCRAPER_MAX_DEPTH")
	viper.BindEnv("mcp.name", "BAMRAG_MCP_NAME")
	viper.BindEnv("mcp.version", "BAMRAG_MCP_VERSION")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Warn("config file error", "error", err)
		}
		// No config file - use defaults + env vars
	}

	// Unmarshal into struct (merges config file with defaults)
	if err := viper.Unmarshal(&cfg); err != nil {
		slog.Warn("failed to parse config", "error", err)
	}

	// Handle special case: addresses as comma-separated string from env
	if addrs := os.Getenv("BAMRAG_ELASTICSEARCH_ADDRESSES"); addrs != "" {
		cfg.Elasticsearch.Addresses = strings.Split(addrs, ",")
	}
}
