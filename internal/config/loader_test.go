package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	os.Setenv("NEO4J_USER", "neo4j")
	os.Setenv("NEO4J_PASSWORD", "password")

	defer func() {
		// Clean up environment variables
		os.Unsetenv("NEO4J_URI")
		os.Unsetenv("NEO4J_USER")
		os.Unsetenv("NEO4J_PASSWORD")
	}()

	cfg := LoadConfig()

	if cfg.Neo4jURI != "bolt://localhost:7687" {
		t.Errorf("expected Neo4jURI to be 'bolt://localhost:7687', got '%s'", cfg.Neo4jURI)
	}
	if cfg.Neo4jUser != "neo4j" {
		t.Errorf("expected Neo4jUser to be 'neo4j', got '%s'", cfg.Neo4jUser)
	}
	if cfg.Neo4jPassword != "password" {
		t.Errorf("expected Neo4jPassword to be 'password', got '%s'", cfg.Neo4jPassword)
	}
}
