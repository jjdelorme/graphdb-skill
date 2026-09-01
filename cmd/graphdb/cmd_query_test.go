//go:build test_mocks

package main

import (
	"context"
	"os"
	"testing"
)

// Tests for CLI queries would go here.
// Semantic Seams CLI tests removed and will be added back in Task 4.3 when the CLI is wired.

func TestHandleQuery_Basic(t *testing.T) {
	// 1. Setup Environment for Mocking
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687") // Just to pass the check
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	// 2. Call handleQuery with status
	args := []string{"-type", "status"}
	
	// This should not panic or exit if mocking is working
	handleQuery(args)
}

func TestHandleQuery_SemanticSeams(t *testing.T) {
	// 1. Setup Environment for Mocking
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	// 2. Call handleQuery with semantic-seams type and similarity
	args := []string{"-type", "semantic-seams", "-similarity", "0.7"}

	// Note: since handleQuery uses flag.ExitOnError, this test will fail if flag parsing fails.
	// But it should also fail if "semantic-seams" is not handled because it will call log.Fatalf.
	// In Go tests, log.Fatalf will exit the process, which will make the test fail with an error.
	handleQuery(args)
}

func TestHandleQuery_DirFlag(t *testing.T) {
	// 1. Setup Environment for Mocking
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	// 2. Setup temporary directory to change into
	tempDir := t.TempDir()
	
	// 3. Save original working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer func() {
		// Restore original working directory after the test
		if err := os.Chdir(origWd); err != nil {
			t.Errorf("Failed to restore original working directory: %v", err)
		}
	}()

	// 4. Call handleQuery with dir flag
	args := []string{"-type", "status", "-dir", tempDir}
	handleQuery(args)
	
	// 5. Verify the working directory has changed
	newWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get new working directory: %v", err)
	}
	
	// Because TempDir on some OS (like Mac) can return a symlink path, 
	// we check if the newWd is the tempDir or resolves to the same path.
	// But simply checking we moved away from origWd is a basic assertion.
	if newWd == origWd {
		t.Errorf("Expected working directory to change, but it remained %s", origWd)
	}
}

func TestMockProvider_GetSemanticSeams(t *testing.T) {
	// Task 4.1 requirement: verify the mock provider can execute and return results for semantic seam detection.
	mock := &MockProvider{}
	ctx := context.Background()
	threshold := 0.5

	results, err := mock.GetSemanticSeams(ctx, threshold)
	if err != nil {
		t.Fatalf("Expected no error from mock, got %v", err)
	}

	if !mock.GetSemanticSeamsCalled {
		t.Errorf("Expected GetSemanticSeamsCalled to be true")
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result from mock, got %d", len(results))
	}

	if results[0].Container != "mock_file.go" {
		t.Errorf("Expected container 'mock_file.go', got '%s'", results[0].Container)
	}

	if results[0].Similarity != 0.1 {
		t.Errorf("Expected similarity 0.1, got %f", results[0].Similarity)
	}
}

func TestHandleQuery_DualLensSeams(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	args := []string{"-type", "seams", "--dual-lens", "-min-score", "10.0", "-max-cut-edges", "4", "-limit", "10"}
	handleQuery(args)
}

func TestHandleQuery_Divergence(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	args := []string{"-type", "divergence", "-domain", "Billing"}
	handleQuery(args)
}

func TestHandleQuery_Communities(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	args := []string{"-type", "communities", "-limit", "25"}
	handleQuery(args)
}

func TestMockProvider_DualLensMethods(t *testing.T) {
	mock := &MockProvider{}
	ctx := context.Background()

	seams, err := mock.GetDualLensSeams(ctx, ".*", 10.0, 4, 20)
	if err != nil || len(seams) == 0 {
		t.Fatalf("Expected non-empty dual lens seams from mock, got %v, err=%v", seams, err)
	}
	if seams[0].ID != "func_mock_1" || seams[0].Score < 10.0 {
		t.Errorf("Unexpected seam from mock: %+v", seams[0])
	}

	div, err := mock.GetDivergence(ctx, "Billing")
	if err != nil || len(div) == 0 {
		t.Fatalf("Expected non-empty divergence from mock, got %v, err=%v", div, err)
	}
	if div[0].DomainName != "Billing" {
		t.Errorf("Unexpected domain name: %s", div[0].DomainName)
	}

	comms, err := mock.GetCommunities(ctx, 10)
	if err != nil || len(comms) == 0 {
		t.Fatalf("Expected non-empty communities from mock, got %v, err=%v", comms, err)
	}
	if comms[0].ID != "comm-1" {
		t.Errorf("Unexpected community ID: %s", comms[0].ID)
	}

	if err := mock.ClearStructuralTopology(); err != nil {
		t.Errorf("Unexpected error clearing structural topology: %v", err)
	}
	if !mock.ClearStructuralTopologyCalled {
		t.Error("Expected ClearStructuralTopologyCalled to be true")
	}

	if err := mock.UpdateStructuralTopology(nil, nil); err != nil {
		t.Errorf("Unexpected error updating structural topology: %v", err)
	}
	if !mock.UpdateStructuralTopologyCalled {
		t.Error("Expected UpdateStructuralTopologyCalled to be true")
	}
}
