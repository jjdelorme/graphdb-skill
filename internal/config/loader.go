package config

import (
	"os"
)

// Config holds the configuration for the graph database connection.
type Config struct {
	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string
}

// LoadConfig loads the configuration from environment variables.
func LoadConfig() Config {
	return Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}
}
