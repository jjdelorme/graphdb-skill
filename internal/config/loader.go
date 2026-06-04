package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the configuration for the graph database connection.
type Config struct {
	Neo4jURI                  string
	Neo4jUser                 string
	Neo4jPassword             string
	Neo4jDatabase             string
	BaseDir                   string
	GoogleCloudProject        string
	GoogleCloudLocation       string
	GeminiEmbeddingModel      string
	GeminiEmbeddingDimensions int
	GeminiGenerativeModel     string
	GenAIBackend              string
	GenAIBaseURL              string
	GenAIAPIKey               string
	GenAIAPIVersion           string
	LLMConcurrency            int
	GeminiBatchGCSBucket      string
}

// LoadConfig loads the configuration from environment variables.
func LoadConfig() Config {
	dimsStr := os.Getenv("GEMINI_EMBEDDING_DIMENSIONS")
	dims, err := strconv.Atoi(dimsStr)
	if err != nil || dims <= 0 {
		dims = 768 // Default for gemini-embedding-001
	}

	concurrencyStr := os.Getenv("LLM_CONCURRENCY")
	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil || concurrency <= 0 {
		concurrency = 5 // Default concurrency
	}

	dbName := os.Getenv("NEO4J_DATABASE")
	if dbName == "" {
		dbName = "neo4j"
	}

	baseDir := os.Getenv("GRAPHDB_DIR")
	if baseDir == "" {
		baseDir = "."
	}

	return Config{
		Neo4jURI:                  os.Getenv("NEO4J_URI"),
		Neo4jUser:                 os.Getenv("NEO4J_USER"),
		Neo4jPassword:             os.Getenv("NEO4J_PASSWORD"),
		Neo4jDatabase:             dbName,
		BaseDir:                   baseDir,
		GoogleCloudProject:        os.Getenv("GOOGLE_CLOUD_PROJECT"),
		GoogleCloudLocation:       os.Getenv("GOOGLE_CLOUD_LOCATION"),
		GeminiEmbeddingModel:      os.Getenv("GEMINI_EMBEDDING_MODEL"),
		GeminiEmbeddingDimensions: dims,
		GeminiGenerativeModel:     os.Getenv("GEMINI_GENERATIVE_MODEL"),
		GenAIBackend:              os.Getenv("GENAI_BACKEND"),
		GenAIBaseURL:              os.Getenv("GENAI_BASE_URL"),
		GenAIAPIKey:               os.Getenv("GENAI_API_KEY"),
		GenAIAPIVersion:           os.Getenv("GENAI_API_VERSION"),
		LLMConcurrency:            concurrency,
		GeminiBatchGCSBucket:      os.Getenv("GEMINI_BATCH_GCS_BUCKET"),
	}
}

// LoadEnv loads environment variables from a .env file, searching up the directory tree.
func LoadEnv() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			// Found it
			return godotenv.Load(envPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	// Not found is fine
	return nil
}
