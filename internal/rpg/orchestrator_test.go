package rpg

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/query"
	"sync"
	"testing"
)

type MockGraphProvider struct {
	GetUnextractedFunctionsFn    func(limit int) ([]*graph.Node, error)
	CountUnextractedFunctionsFn func() (int64, error)
	UpdateAtomicFeaturesFn      func(id string, features []string, isVolatile bool) error
	GetUnembeddedNodesFn      func(limit int) ([]*graph.Node, error)
	CountUnembeddedNodesFn    func() (int64, error)
	UpdateEmbeddingsFn        func(id string, embedding []float32) error
	GetEmbeddingsOnlyFn       func() (map[string][]float32, error)
	GetUnnamedFeaturesFn      func(limit int) ([]*graph.Node, error)
	CountUnnamedFeaturesFn    func() (int64, error)
	ClearFeatureTopologyFn    func() error
	UpdateFeatureTopologyFn   func(nodes []*graph.Node, edges []*graph.Edge) error
	UpdateFeatureSummaryFn    func(id string, name string, description string) error
	GetFunctionMetadataFn     func() ([]*graph.Node, error)
	ExploreDomainFn           func(featureID string) (*query.DomainExplorationResult, error)
}

func (m *MockGraphProvider) Close() error { return nil }
func (m *MockGraphProvider) Traverse(startNodeID string, relationship string, direction query.Direction, depth int) ([]*graph.Path, error) {
	return nil, nil
}
func (m *MockGraphProvider) RunCypher(cypherQuery string) ([]map[string]any, error) {
	return nil, nil
}
func (m *MockGraphProvider) FindDuplicates(similarityThreshold float64, limit int) ([]*query.DuplicateResult, error) {
	return nil, nil
}

func (m *MockGraphProvider) SearchFeatures(queryStr string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) SearchSimilarFunctions(query string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetNeighbors(nodeID string, depth int, limit int) (*query.NeighborResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetCallers(nodeID string) ([]string, error) { return nil, nil }
func (m *MockGraphProvider) GetImpact(nodeID string, depth int) (*query.ImpactResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetGlobals(nodeID string) (*query.GlobalUsageResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetSeams(modulePattern string, layer string) ([]*query.SeamResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetHotspots(modulePattern string) ([]*query.HotspotResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) FetchSource(nodeID string) (string, error) { return "", nil }
func (m *MockGraphProvider) LocateUsage(sourceID string, targetID string) (any, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetGraphState() (string, error)                       { return "", nil }
func (m *MockGraphProvider) SemanticTrace(nodeID string) ([]*graph.Path, error)   { return nil, nil }
func (m *MockGraphProvider) GetOverview() (*graph.Path, error)                    { return nil, nil }
func (m *MockGraphProvider) GetStats() (map[string]int64, error)               { return nil, nil }
func (m *MockGraphProvider) WhatIf(targets []string) (*query.WhatIfResult, error) { return nil, nil }
func (m *MockGraphProvider) GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*query.SemanticSeamResult, error) {
	return nil, nil
}

func (m *MockGraphProvider) GetCoverage(nodeID string) ([]*graph.Node, error) { return nil, nil }
func (m *MockGraphProvider) LinkTests() error                                 { return nil }

func (m *MockGraphProvider) SeedVolatility(modulePattern string, rules []query.ContaminationRule) error {
	return nil
}
func (m *MockGraphProvider) PropagateVolatility() error { return nil }
func (m *MockGraphProvider) CalculateRiskScores() error { return nil }
func (m *MockGraphProvider) CountVolatileFunctions() (int64, error) { return 0, nil }
func (m *MockGraphProvider) UpdateFileHistory(metrics map[string]query.FileHistoryMetrics) error {
	return nil
}

func (m *MockGraphProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error) {
	if m.GetUnextractedFunctionsFn != nil {
		return m.GetUnextractedFunctionsFn(limit)
	}
	return nil, nil
}
func (m *MockGraphProvider) CountUnextractedFunctions() (int64, error) {
	if m.CountUnextractedFunctionsFn != nil {
		return m.CountUnextractedFunctionsFn()
	}
	return 0, nil
}
func (m *MockGraphProvider) UpdateAtomicFeatures(id string, features []string, isVolatile bool) error {
	if m.UpdateAtomicFeaturesFn != nil {
		return m.UpdateAtomicFeaturesFn(id, features, isVolatile)
	}
	return nil
}
func (m *MockGraphProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error) {
	if m.GetUnembeddedNodesFn != nil {
		return m.GetUnembeddedNodesFn(limit)
	}
	return nil, nil
}
func (m *MockGraphProvider) CountUnembeddedNodes() (int64, error) {
	if m.CountUnembeddedNodesFn != nil {
		return m.CountUnembeddedNodesFn()
	}
	return 0, nil
}
func (m *MockGraphProvider) UpdateEmbeddings(id string, embedding []float32) error {
	if m.UpdateEmbeddingsFn != nil {
		return m.UpdateEmbeddingsFn(id, embedding)
	}
	return nil
}
func (m *MockGraphProvider) GetEmbeddingsOnly() (map[string][]float32, error) {
	if m.GetEmbeddingsOnlyFn != nil {
		return m.GetEmbeddingsOnlyFn()
	}
	return nil, nil
}
func (m *MockGraphProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error) {
	if m.GetUnnamedFeaturesFn != nil {
		return m.GetUnnamedFeaturesFn(limit)
	}
	return nil, nil
}
func (m *MockGraphProvider) CountUnnamedFeatures() (int64, error) {
	if m.CountUnnamedFeaturesFn != nil {
		return m.CountUnnamedFeaturesFn()
	}
	return 0, nil
}
func (m *MockGraphProvider) ClearFeatureTopology() error {
	if m.ClearFeatureTopologyFn != nil {
		return m.ClearFeatureTopologyFn()
	}
	return nil
}
func (m *MockGraphProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	if m.UpdateFeatureTopologyFn != nil {
		return m.UpdateFeatureTopologyFn(nodes, edges)
	}
	return nil
}
func (m *MockGraphProvider) UpdateFeatureSummary(id string, name string, description string) error {
	if m.UpdateFeatureSummaryFn != nil {
		return m.UpdateFeatureSummaryFn(id, name, description)
	}
	return nil
}

func (m *MockGraphProvider) GetFunctionMetadata() ([]*graph.Node, error) {
	if m.GetFunctionMetadataFn != nil {
		return m.GetFunctionMetadataFn()
	}
	return nil, nil
}
func (m *MockGraphProvider) ExploreDomain(featureID string) (*query.DomainExplorationResult, error) {
	if m.ExploreDomainFn != nil {
		return m.ExploreDomainFn(featureID)
	}
	return nil, nil
}

func (m *MockGraphProvider) CreateBatchJobNode(ctx context.Context, jobID, modelName, gcsInput, gcsOutput string) error {
	return nil
}
func (m *MockGraphProvider) UpdateBatchJobNodeStatus(ctx context.Context, jobID, state, failureReason string) error {
	return nil
}
func (m *MockGraphProvider) GetActiveBatchJobs(ctx context.Context) ([]query.BatchJob, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetBatchJobCount(ctx context.Context) (int, error) {
	return 0, nil
}

func TestOrchestratorExtraction(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnextractedFunctionsFn = func() (int64, error) { return 1, nil }

	callCount := 0
	mockProvider.GetUnextractedFunctionsFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		return []*graph.Node{
			{ID: "f1", Properties: map[string]any{"name": "testFunc", "file": "test.go", "start_line": 1, "end_line": 2}},
		}, nil
	}

	updateCount := 0
	mockProvider.UpdateAtomicFeaturesFn = func(id string, features []string, isVolatile bool) error {
		updateCount++
		if id != "f1" {
			t.Errorf("Expected f1, got %s", id)
		}
		return nil
	}

	extractor := &MockFeatureExtractor{}

	orchestrator := &Orchestrator{
		Provider:  mockProvider,
		Extractor: extractor,
	}

	err := orchestrator.RunExtraction(10, ".")
	if err != nil {
		t.Fatalf("RunExtraction failed: %v", err)
	}

	if updateCount != 1 {
		t.Errorf("Expected 1 update, got %d", updateCount)
	}
}

func TestOrchestratorEmbedding(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnembeddedNodesFn = func() (int64, error) { return 1, nil }

	callCount := 0
	mockProvider.GetUnembeddedNodesFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		return []*graph.Node{
			{ID: "f1", Label: "Function", Properties: map[string]any{"name": "testFunc"}},
		}, nil
	}

	updateCount := 0
	mockProvider.UpdateEmbeddingsFn = func(id string, embedding []float32) error {
		updateCount++
		if id != "f1" {
			t.Errorf("Expected f1, got %s", id)
		}
		return nil
	}

	orchestrator := &Orchestrator{
		Provider: mockProvider,
		Embedder: &MockEmbedder{},
	}

	err := orchestrator.RunEmbedding(10)
	if err != nil {
		t.Fatalf("RunEmbedding failed: %v", err)
	}

	if updateCount != 1 {
		t.Errorf("Expected 1 update, got %d", updateCount)
	}
}

func TestOrchestratorClustering(t *testing.T) {
	mockProvider := &MockGraphProvider{}

	// Setup embeddings
	mockProvider.GetEmbeddingsOnlyFn = func() (map[string][]float32, error) {
		return map[string][]float32{
			"f1": {0.1},
			"f2": {0.2},
		}, nil
	}

	// Setup function metadata
	mockProvider.GetFunctionMetadataFn = func() ([]*graph.Node, error) {
		return []*graph.Node{
			{ID: "f1", Properties: map[string]any{"file": "file1.go"}},
			{ID: "f2", Properties: map[string]any{"file": "file2.go"}},
		}, nil
	}

	updateTopologyCalled := false
	mockProvider.UpdateFeatureTopologyFn = func(nodes []*graph.Node, edges []*graph.Edge) error {
		updateTopologyCalled = true
		return nil
	}

	orchestrator := &Orchestrator{
		Provider:   mockProvider,
		Embedder:   &MockEmbedder{},
		Summarizer: &MockSummarizer{},
		Seed:       42,
	}

	err := orchestrator.RunClustering(".")
	if err != nil {
		t.Fatalf("RunClustering failed: %v", err)
	}

	if !updateTopologyCalled {
		t.Errorf("Expected UpdateFeatureTopology to be called")
	}
}

func TestCalculateDomainK(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		expected  int
	}{
		{"Zero files", 0, 0},
		{"Very small (hits floor)", 10, 5},
		{"Small (hits floor)", 50, 5},
		{"Small-Med", 200, 6},
		{"Medium", 1000, 14},
		{"Medium-Large", 2000, 20},
		{"Large", 5000, 31},
		{"Very Large", 10000, 44},
		{"Massive (hits ceiling)", 33000, 50},
		{"Excessive (stays at ceiling)", 100000, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDomainK(tt.fileCount)
			if result != tt.expected {
				t.Errorf("CalculateDomainK(%d) = %d; expected %d", tt.fileCount, result, tt.expected)
			}
		})
	}
}

func TestOrchestratorExtraction_HappyPath(t *testing.T) {
	// This test asserts that Orchestrator reads the correct 'start_line' property
	// as per the canonical schema.

	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnextractedFunctionsFn = func() (int64, error) { return 1, nil }

	callCount := 0
	mockProvider.GetUnextractedFunctionsFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		return []*graph.Node{
			{
				ID: "f1",
				Properties: map[string]any{
					"name":       "testFunc",
					"file":       "test.go",
					"start_line": 10,
					"end_line":   20,
				},
			},
		}, nil
	}

	var updatedFeatures []string
	mockProvider.UpdateAtomicFeaturesFn = func(id string, features []string, isVolatile bool) error {
		updatedFeatures = features
		return nil
	}

	orchestrator := &Orchestrator{
		Provider:  mockProvider,
		Extractor: &MockFeatureExtractor{Descriptors: []string{"real", "feature"}},
		Loader: func(path string, start, end int) (string, error) {
			return "func testFunc() {}", nil
		},
	}

	err := orchestrator.RunExtraction(10, ".")
	if err != nil {
		t.Fatalf("RunExtraction failed: %v", err)
	}
	
	if len(updatedFeatures) == 0 {
		t.Errorf("UpdateAtomicFeatures not called")
	} else if updatedFeatures[0] == "unknown" {
		t.Errorf("Expected real features, but got 'unknown' fallback. Ensure 'start_line' is correctly read.")
	} else if updatedFeatures[0] != "real" {
		t.Errorf("Expected 'real' feature, got %v", updatedFeatures[0])
	}
}

func TestOrchestratorExtraction_AbortsOnFirstError(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnextractedFunctionsFn = func() (int64, error) { return 6, nil }

	var nodes []*graph.Node
	for i := 0; i < 6; i++ {
		nodes = append(nodes, &graph.Node{ID: "f" + string(rune(i+'0')), Properties: map[string]any{"name": "testFunc", "file": "test.go", "start_line": int(1), "end_line": int(2)}})
	}

	callCount := 0
	mockProvider.GetUnextractedFunctionsFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		return nodes, nil
	}

	updateCount := 0
	mockProvider.UpdateAtomicFeaturesFn = func(id string, features []string, isVolatile bool) error {
		updateCount++
		return nil
	}

	extractor := &MockFeatureExtractor{Err: fmt.Errorf("mock error")}

	orchestrator := &Orchestrator{
		Provider:  mockProvider,
		Extractor: extractor,
		Loader: func(path string, start, end int) (string, error) {
			return "func testFunc() {}", nil
		},
	}

	err := orchestrator.RunExtraction(10, ".")
	if err == nil {
		t.Fatalf("RunExtraction should have failed")
	}

	if updateCount != 0 {
		t.Errorf("Expected 0 updates on hard failure, got %d", updateCount)
	}

	expectedErr := "extraction failed for f0: mock error"
	if err.Error() != expectedErr {
		t.Errorf("Unexpected error message: %v; expected: %v", err, expectedErr)
	}
}

func TestOrchestratorSummarization_AbortsOnFirstError(t *testing.T) {
	mockProvider := &MockGraphProvider{}

	var nodes []*graph.Node
	for i := 0; i < 6; i++ {
		nodes = append(nodes, &graph.Node{ID: "f" + string(rune(i+'0')), Label: "Domain", Properties: map[string]any{"name": "testFunc"}})
	}

	callCount := 0
	mockProvider.GetUnnamedFeaturesFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		return nodes, nil
	}

	mockProvider.CountUnnamedFeaturesFn = func() (int64, error) {
		return 6, nil
	}

	updateCount := 0
	mockProvider.UpdateFeatureSummaryFn = func(id string, name string, description string) error {
		updateCount++
		return nil
	}

	mockProvider.ExploreDomainFn = func(featureID string) (*query.DomainExplorationResult, error) {
		return &query.DomainExplorationResult{Functions: []*graph.Node{{ID: "f1", Properties: map[string]any{"file": "test.go", "start_line": int(1), "end_line": int(2)}}}}, nil
	}

	summarizer := &MockSummarizer{SummarizeFunc: func(snippets []string, level string) (string, string, error) {
		return "", "", fmt.Errorf("mock summarizer error")
	}}

	orchestrator := &Orchestrator{
		Provider:   mockProvider,
		Summarizer: summarizer,
		Loader: func(path string, start, end int) (string, error) {
			return "func testFunc() {}", nil
		},
	}

	err := orchestrator.RunSummarization(10, ".")
	if err == nil {
		t.Fatalf("RunSummarization should have failed")
	}

	if updateCount != 0 {
		t.Errorf("Expected 0 updates on hard failure, got %d", updateCount)
	}

	expectedErr := "summarization failed for f0 (Domain): mock summarizer error"
	if err.Error() != expectedErr {
		t.Errorf("Unexpected error message: %v; expected: %v", err, expectedErr)
	}
}

func TestOrchestrator_RunExtraction_Concurrent(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnextractedFunctionsFn = func() (int64, error) { return 5, nil }

	var mu sync.Mutex
	callCount := 0
	mockProvider.GetUnextractedFunctionsFn = func(limit int) ([]*graph.Node, error) {
		mu.Lock()
		defer mu.Unlock()
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		var nodes []*graph.Node
		for i := 0; i < 5; i++ {
			nodes = append(nodes, &graph.Node{
				ID: fmt.Sprintf("f%d", i),
				Properties: map[string]any{
					"name":       "testFunc",
					"file":       "test.go",
					"start_line": 1,
					"end_line":   2,
				},
			})
		}
		return nodes, nil
	}

	updateCount := 0
	mockProvider.UpdateAtomicFeaturesFn = func(id string, features []string, isVolatile bool) error {
		mu.Lock()
		defer mu.Unlock()
		updateCount++
		return nil
	}

	orchestrator := &Orchestrator{
		Provider:       mockProvider,
		Extractor:      &MockFeatureExtractor{},
		Loader:         func(path string, start, end int) (string, error) { return "code", nil },
		LLMConcurrency: 5,
	}

	err := orchestrator.RunExtraction(10, ".")
	if err != nil {
		t.Fatalf("RunExtraction failed: %v", err)
	}

	if updateCount != 5 {
		t.Errorf("Expected 5 updates, got %d", updateCount)
	}
}

func TestOrchestrator_RunSummarization_Concurrent(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	mockProvider.CountUnnamedFeaturesFn = func() (int64, error) { return 5, nil }

	var mu sync.Mutex
	callCount := 0
	mockProvider.GetUnnamedFeaturesFn = func(limit int) ([]*graph.Node, error) {
		mu.Lock()
		defer mu.Unlock()
		if callCount > 0 {
			return nil, nil
		}
		callCount++
		var nodes []*graph.Node
		for i := 0; i < 5; i++ {
			nodes = append(nodes, &graph.Node{
				ID:    fmt.Sprintf("feat%d", i),
				Label: "Feature",
				Properties: map[string]any{
					"name": "Cluster-001",
				},
			})
		}
		return nodes, nil
	}

	mockProvider.ExploreDomainFn = func(featureID string) (*query.DomainExplorationResult, error) {
		return &query.DomainExplorationResult{
			Functions: []*graph.Node{
				{
					Properties: map[string]any{
						"file":       "test.go",
						"start_line": 1,
						"end_line":   2,
					},
				},
			},
		}, nil
	}

	updateCount := 0
	mockProvider.UpdateFeatureSummaryFn = func(id string, name string, description string) error {
		mu.Lock()
		defer mu.Unlock()
		updateCount++
		return nil
	}

	orchestrator := &Orchestrator{
		Provider:       mockProvider,
		Summarizer:     &MockSummarizer{},
		Loader:         func(path string, start, end int) (string, error) { return "code", nil },
		LLMConcurrency: 5,
	}

	err := orchestrator.RunSummarization(10, ".")
	if err != nil {
		t.Fatalf("RunSummarization failed: %v", err)
	}

	if updateCount != 5 {
		t.Errorf("Expected 5 updates, got %d", updateCount)
	}
}
