//go:build test_mocks

package main

import (
	"context"
	"graphdb/internal/config"
	"graphdb/internal/query"
	"os"
	"reflect"
	"testing"
)

func TestHandleBuildAll_ImportsBothGraphs(t *testing.T) {
	// 1. Setup Mocks
	var ingestCalledWith []string
	var enrichCalledWith []string
	var enrichHistoryCalledWith []string
	var enrichContaminationCalledWith []string
	var enrichTestsCalledWith []string
	var importCalls [][]string

	// Swap handlers
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd
	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
	}()

	ingestCmd = func(args []string) {
		ingestCalledWith = args
	}
	enrichCmd = func(args []string) {
		enrichCalledWith = args
	}
	importCmd = func(args []string) {
		importCalls = append(importCalls, args)
	}
	enrichHistoryCmd = func(args []string) {
		enrichHistoryCalledWith = args
	}
	enrichContaminationCmd = func(args []string) {
		enrichContaminationCalledWith = args
	}
	enrichTestsCmd = func(args []string) {
		enrichTestsCalledWith = args
	}

	// 2. Run handleBuildAll with default flags (clean=true by default)
	args := []string{"-dir", "test_project"}
	handleBuildAll(args)

	// 3. Assertions

	// Verify Ingest
	expectedIngest := []string{"-dir", "test_project", "-nodes", "nodes.jsonl", "-edges", "edges.jsonl"}
	if !reflect.DeepEqual(ingestCalledWith, expectedIngest) {
		t.Errorf("Ingest args mismatch.\nGot: %v\nWant: %v", ingestCalledWith, expectedIngest)
	}

	// Verify Enrich
	expectedEnrich := []string{"-dir", "test_project"}
	if !reflect.DeepEqual(enrichCalledWith, expectedEnrich) {
		t.Errorf("Enrich args mismatch.\nGot: %v\nWant: %v", enrichCalledWith, expectedEnrich)
	}

	// Verify Enrich History
	expectedEnrichHistory := []string{"-dir", "test_project"}
	if !reflect.DeepEqual(enrichHistoryCalledWith, expectedEnrichHistory) {
		t.Errorf("Enrich History args mismatch.\nGot: %v\nWant: %v", enrichHistoryCalledWith, expectedEnrichHistory)
	}

	// Verify Enrich Contamination
	expectedEnrichContamination := []string{}
	if !reflect.DeepEqual(enrichContaminationCalledWith, expectedEnrichContamination) {
		t.Errorf("Enrich Contamination args mismatch.\nGot: %v\nWant: %v", enrichContaminationCalledWith, expectedEnrichContamination)
	}

	// Verify Enrich Tests
	expectedEnrichTests := []string{}
	if !reflect.DeepEqual(enrichTestsCalledWith, expectedEnrichTests) {
		t.Errorf("Enrich Tests args mismatch.\nGot: %v\nWant: %v", enrichTestsCalledWith, expectedEnrichTests)
	}

	// Verify Import calls
	// We expect 1 import call now (Structural graph)
	if len(importCalls) != 1 {
		t.Fatalf("Expected 1 import call, got %d", len(importCalls))
	}

	// Check Call 1: Structural
	expectedImport1 := []string{"-nodes", "nodes.jsonl", "-edges", "edges.jsonl"}
	if !reflect.DeepEqual(importCalls[0], expectedImport1) {
		t.Errorf("First import call mismatch.\nGot: %v\nWant: %v", importCalls[0], expectedImport1)
	}
}

func TestHandleBuildAll_CleansUpIntermediateFiles(t *testing.T) {
	// Setup Mocks
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd

	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
	}()

	ingestCmd = func(args []string) {}
	enrichCmd = func(args []string) {}
	importCmd = func(args []string) {}
	enrichHistoryCmd = func(args []string) {}
	enrichContaminationCmd = func(args []string) {}
	enrichTestsCmd = func(args []string) {}

	// Create dummy JSONL files
	nodesFile := "test_nodes.jsonl"
	edgesFile := "test_edges.jsonl"

	if err := os.WriteFile(nodesFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create %s: %v", nodesFile, err)
	}
	if err := os.WriteFile(edgesFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create %s: %v", edgesFile, err)
	}

	defer func() {
		os.Remove(nodesFile)
		os.Remove(edgesFile)
	}()

	// Run handleBuildAll
	args := []string{"-nodes", nodesFile, "-edges", edgesFile}
	handleBuildAll(args)

	// Assert that files are gone
	if _, err := os.Stat(nodesFile); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be deleted, but it still exists", nodesFile)
	}
	if _, err := os.Stat(edgesFile); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be deleted, but it still exists", edgesFile)
	}
}

func TestHandleBuildAll_BatchMode(t *testing.T) {
	t.Setenv("GRAPHDB_MOCK_ENABLED", "true")

	var ingestCalled bool
	var enrichCalledWith []string
	var enrichHistoryCalled bool
	var enrichContaminationCalled bool
	var enrichTestsCalled bool

	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd

	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
	}()

	ingestCmd = func(args []string) { ingestCalled = true }
	enrichCmd = func(args []string) { enrichCalledWith = args }
	importCmd = func(args []string) { }
	enrichHistoryCmd = func(args []string) { enrichHistoryCalled = true }
	enrichContaminationCmd = func(args []string) { enrichContaminationCalled = true }
	enrichTestsCmd = func(args []string) { enrichTestsCalled = true }

	args := []string{"-dir", "test_project", "--batch", "--gcs-bucket", "test-bucket"}
	handleBuildAll(args)

	if !ingestCalled {
		t.Error("Expected ingest to be called")
	}

	expectedEnrich := []string{"-dir", "test_project", "--batch", "--gcs-bucket", "test-bucket"}
	if !reflect.DeepEqual(enrichCalledWith, expectedEnrich) {
		t.Errorf("Enrich args mismatch.\nGot: %v\nWant: %v", enrichCalledWith, expectedEnrich)
	}

	if enrichHistoryCalled || enrichContaminationCalled || enrichTestsCalled {
		t.Error("Expected remaining phases to NOT be called in batch mode")
	}
}

func TestHandleBuildAll_ResumeMode(t *testing.T) {
	t.Setenv("GRAPHDB_MOCK_ENABLED", "true")

	var checkBatchCalledWith []string
	var enrichHistoryCalled bool
	var enrichContaminationCalled bool
	var enrichTestsCalled bool

	originalEnrich := enrichCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd
	originalSetupProvider := setupProviderFn

	defer func() {
		enrichCmd = originalEnrich
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
		setupProviderFn = originalSetupProvider
	}()

	var enrichCalls [][]string
	enrichCmd = func(args []string) { enrichCalls = append(enrichCalls, args) }
	enrichHistoryCmd = func(args []string) { enrichHistoryCalled = true }
	enrichContaminationCmd = func(args []string) { enrichContaminationCalled = true }
	enrichTestsCmd = func(args []string) { enrichTestsCalled = true }
 
	// Mock provider that has 1 completed batch job and 0 active ones
	mockProvider := &MockProvider{
		BatchJobCount:   1,
		ActiveBatchJobs: nil,
	}
	setupProviderFn = func(cfg config.Config) (query.GraphProvider, error) {
		return mockProvider, nil
	}
 
	args := []string{"-dir", "test_project", "--resume"}
	handleBuildAll(args)
 
	if len(enrichCalls) != 2 {
		t.Errorf("Expected enrichCmd to be called exactly twice, got %d times", len(enrichCalls))
	} else {
		expectedCheckBatch := []string{"--check-batch"}
		if !reflect.DeepEqual(enrichCalls[0], expectedCheckBatch) {
			t.Errorf("First enrich call args mismatch.\nGot: %v\nWant: %v", enrichCalls[0], expectedCheckBatch)
		}
		expectedLocalEnrich := []string{"-dir", "test_project"}
		if !reflect.DeepEqual(enrichCalls[1], expectedLocalEnrich) {
			t.Errorf("Second enrich call args mismatch.\nGot: %v\nWant: %v", enrichCalls[1], expectedLocalEnrich)
		}
	}

	// Verify remaining phases WERE called (as active batch jobs is 0)
	if !enrichHistoryCalled {
		t.Error("Expected enrichHistoryCmd to be called upon resume completion")
	}
	if !enrichContaminationCalled {
		t.Error("Expected enrichContaminationCmd to be called upon resume completion")
	}
	if !enrichTestsCalled {
		t.Error("Expected enrichTestsCmd to be called upon resume completion")
	}
}

func TestHandleBuildAll_CleanMode(t *testing.T) {
	t.Setenv("GRAPHDB_MOCK_ENABLED", "true")

	var ingestCalledWith []string
	var enrichCalled bool
	var importCalled bool
	var wipeCalled bool

	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd
	originalSetupProvider := setupProviderFn

	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
		setupProviderFn = originalSetupProvider
	}()

	ingestCmd = func(args []string) { ingestCalledWith = args }
	enrichCmd = func(args []string) { enrichCalled = true }
	importCmd = func(args []string) { importCalled = true }
	enrichHistoryCmd = func(args []string) {}
	enrichContaminationCmd = func(args []string) {}
	enrichTestsCmd = func(args []string) {}

	mockProvider := &MockProvider{
		WipeDatabaseFn: func(ctx context.Context) error {
			wipeCalled = true
			return nil
		},
	}
	setupProviderFn = func(cfg config.Config) (query.GraphProvider, error) {
		return mockProvider, nil
	}

	args := []string{"-dir", "test_project", "--clean"}
	handleBuildAll(args)

	if !wipeCalled {
		t.Error("Expected WipeDatabase to be called on provider in clean mode")
	}

	// Verify it forces a non-incremental build:
	// Ingest must be called with full flags, including -nodes and -edges.
	expectedIngest := []string{"-dir", "test_project", "-nodes", "nodes.jsonl", "-edges", "edges.jsonl"}
	if !reflect.DeepEqual(ingestCalledWith, expectedIngest) {
		t.Errorf("Expected full ingest args %v, got %v", expectedIngest, ingestCalledWith)
	}

	if !importCalled {
		t.Error("Expected import to run in clean mode (non-incremental)")
	}

	if !enrichCalled {
		t.Error("Expected enrich to run in clean mode")
	}
}

