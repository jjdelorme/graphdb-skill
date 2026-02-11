package config

import (
	"os"
	"path/filepath"
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

func TestLoadEnv(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a .env file in the temp directory
	envContent := "TEST_ENV_VAR=loaded_successfully"
	envFile := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir", "deep", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Change working directory to the subdirectory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(wd) // Restore original working directory

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Failed to change working directory: %v", err)
	}

	// Call LoadEnv
	if err := LoadEnv(); err != nil {
		// We expect LoadEnv to potentially return an error if .env is not found in a real scenario,
		// but here we know it should be found.
		// However, for this test to be robust, we should ensure LoadEnv is implemented to search up.
		t.Fatalf("LoadEnv failed: %v", err)
	}

	// Check if the environment variable is loaded
	if val := os.Getenv("TEST_ENV_VAR"); val != "loaded_successfully" {
		t.Errorf("Expected TEST_ENV_VAR to be 'loaded_successfully', got '%s'", val)
	}

	// Cleanup env var
	os.Unsetenv("TEST_ENV_VAR")
}
