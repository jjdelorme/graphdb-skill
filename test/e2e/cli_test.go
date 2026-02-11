package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func getRepoRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Traverse up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("Could not find repo root (go.mod)")
		}
		wd = parent
	}
}

func buildCLI(t *testing.T) string {
	root := getRepoRoot(t)
	outputPath := filepath.Join(root, "bin", "graphdb_test")
	cmdPath := filepath.Join(root, "cmd", "graphdb")

	// Ensure bin directory exists
	os.MkdirAll(filepath.Join(root, "bin"), 0755)

	cmd := exec.Command("go", "build", "-o", outputPath, cmdPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, output)
	}
	return outputPath
}

func TestCLI_Ingest(t *testing.T) {
	cliPath := buildCLI(t)
	root := getRepoRoot(t)
	// Clean up binary after test, or keep it? Keeping it is fine, gitignore should ignore bin/
    // actually, let's remove it in a cleanup function if we want to be clean.
    // defer os.Remove(cliPath) 

	outFile := filepath.Join(root, "test_graph_cli.jsonl")
	defer os.Remove(outFile)

	fixturesPath := filepath.Join(root, "test", "fixtures", "typescript")

	// Run ingest
	cmd := exec.Command(cliPath, "ingest",
		"-dir", fixturesPath,
		"-output", outFile,
		"-mock-embedding",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Ingest command failed: %v\nOutput: %s", err, output)
	}

	// Verify output file exists and has content
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Output file is empty")
	}
}

func TestCLI_Query_Help(t *testing.T) {
	cliPath := buildCLI(t)

	// Test help/unknown command
	cmd := exec.Command(cliPath, "unknown")
	output, err := cmd.CombinedOutput()

	// It should exit with 1
	if err == nil {
		t.Error("Expected error for unknown command, got nil")
	}
	if !strings.Contains(string(output), "Unknown command") {
		t.Errorf("Expected 'Unknown command' message, got: %s", output)
	}
}

func TestCLI_Query_Seams(t *testing.T) {
	if os.Getenv("NEO4J_URI") == "" {
		t.Skip("Skipping query test: NEO4J_URI not set")
	}

	cliPath := buildCLI(t)

	// Run query
	cmd := exec.Command(cliPath, "query", "-type", "seams", "-module", ".*")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Query command failed: %v\nOutput: %s", err, output)
	}

	// Check for JSON output (starts with [ or is null or empty list)
	outStr := strings.TrimSpace(string(output))
	if !strings.HasPrefix(outStr, "[") && outStr != "null" {
		t.Errorf("Expected JSON array output, got: %s", outStr)
	}
}
