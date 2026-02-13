package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds the configuration for the graph database connection.
type Config struct {
	Neo4jURI             string
	Neo4jUser            string
	Neo4jPassword        string
	GoogleCloudProject   string
	GoogleCloudLocation  string
	GeminiEmbeddingModel string
}

// LoadConfig loads the configuration from environment variables.
func LoadConfig() Config {
	return Config{
		Neo4jURI:             os.Getenv("NEO4J_URI"),
		Neo4jUser:            os.Getenv("NEO4J_USER"),
		Neo4jPassword:        os.Getenv("NEO4J_PASSWORD"),
		GoogleCloudProject:   os.Getenv("GOOGLE_CLOUD_PROJECT"),
		GoogleCloudLocation:  os.Getenv("GOOGLE_CLOUD_LOCATION"),
		GeminiEmbeddingModel: os.Getenv("GEMINI_EMBEDDING_MODEL"),
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
