//go:build test_mocks

package main

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/query"
)

// MockEmbedder for testing/dry-run
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768) // Dummy 768-dim vector
	}
	return res, nil
}

// MockSummarizer for placeholder RPG
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(snippets []string, level string, extraContext string) (string, string, error) {
	return "Mock Feature", "Automatically generated description based on " + fmt.Sprintf("%d", len(snippets)) + " snippets.", nil
}

// MockProvider for testing/dry-run
type MockProvider struct {
	GetSemanticSeamsCalled         bool
	WipeDatabaseFn                 func(ctx context.Context) error
	BatchJobCount                  int
	ActiveBatchJobs                []query.BatchJob
	DualLensSeamsResults           []*query.DualLensSeamResult
	DivergenceResults              []*query.DomainDivergenceResult
	CommunitiesResults             []*query.StructuralCommunityResult
	ClearStructuralTopologyCalled  bool
	UpdateStructuralTopologyCalled bool
}

func (m *MockProvider) Close() error { return nil }
func (m *MockProvider) WipeDatabase(ctx context.Context) error {
	if m.WipeDatabaseFn != nil {
		return m.WipeDatabaseFn(ctx)
	}
	return nil
}

func (m *MockProvider) Traverse(startNodeID string, relationship string, direction query.Direction, depth int) ([]*graph.Path, error) {
	return nil, nil
}
func (m *MockProvider) RunCypher(query string) ([]map[string]any, error) {
	return nil, nil
}
func (m *MockProvider) SearchFeatures(query string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
	return nil, nil
}
func (m *MockProvider) SearchSimilarFunctions(query string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
	return nil, nil
}
func (m *MockProvider) FindDuplicates(similarityThreshold float64, limit int) ([]*query.DuplicateResult, error) {
	return nil, nil
}
func (m *MockProvider) GetNeighbors(nodeID string, depth int, limit int) (*query.NeighborResult, error) {
	return nil, nil
}
func (m *MockProvider) GetCallers(nodeID string) ([]string, error) { return nil, nil }
func (m *MockProvider) GetImpact(nodeID string, depth int) (*query.ImpactResult, error) {
	return nil, nil
}
func (m *MockProvider) GetGlobals(nodeID string) (*query.GlobalUsageResult, error) { return nil, nil }
func (m *MockProvider) GetSeams(modulePattern string, layer string) ([]*query.SeamResult, error) {
	return nil, nil
}
func (m *MockProvider) GetHotspots(modulePattern string) ([]*query.HotspotResult, error) { return nil, nil }
func (m *MockProvider) FetchSource(nodeID string) (string, error)                 { return "", nil }
func (m *MockProvider) LocateUsage(sourceID string, targetID string) (any, error) { return nil, nil }
func (m *MockProvider) GetOverview() (*graph.Path, error)                         { return &graph.Path{}, nil }
func (m *MockProvider) GetGraphState() (string, error)                            { return "", nil }
func (m *MockProvider) GetStats() (map[string]int64, error)                       { return map[string]int64{"nodes": 0}, nil }
func (m *MockProvider) ExploreDomain(featureID string) (*query.DomainExplorationResult, error) {
	return nil, nil
}
func (m *MockProvider) SemanticTrace(nodeID string) ([]*graph.Path, error) {
	return nil, nil
}
func (m *MockProvider) WhatIf(targets []string) (*query.WhatIfResult, error) { return nil, nil }
func (m *MockProvider) GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*query.SemanticSeamResult, error) {
	m.GetSemanticSeamsCalled = true
	return []*query.SemanticSeamResult{
		{
			Container:  "mock_file.go",
			MethodA:    "funcA",
			MethodB:    "funcB",
			Similarity: 0.1,
		},
	}, nil
}

func (m *MockProvider) GetDualLensSeams(ctx context.Context, modulePattern string, minScore float64, maxCutEdges int, limit int) ([]*query.DualLensSeamResult, error) {
	if m.DualLensSeamsResults != nil {
		return m.DualLensSeamsResults, nil
	}
	return []*query.DualLensSeamResult{
		{
			ID:             "func_mock_1",
			Seam:           "ProcessPayment",
			File:           "payment/gateway.go",
			InternalFanIn:  25,
			VolatileFanOut: 20,
			CutEdges:       2,
			Score:          166.6667,
			Community:      "comm-1",
			Domain:         "Billing",
		},
	}, nil
}

func (m *MockProvider) GetDivergence(ctx context.Context, domainPattern string) ([]*query.DomainDivergenceResult, error) {
	if m.DivergenceResults != nil {
		return m.DivergenceResults, nil
	}
	return []*query.DomainDivergenceResult{
		{
			DomainID:        "domain_billing",
			DomainName:      "Billing",
			TotalFunctions:  10,
			DivergenceScore: 0.3,
			Distribution: []query.CommunityDistributionItem{
				{
					CommunityID:   "comm-1",
					CommunityName: "Community 1",
					FunctionCount: 7,
					Ratio:         0.7,
				},
				{
					CommunityID:   "comm-2",
					CommunityName: "Community 2",
					FunctionCount: 3,
					Ratio:         0.3,
				},
			},
		},
	}, nil
}

func (m *MockProvider) GetCommunities(ctx context.Context, limit int) ([]*query.StructuralCommunityResult, error) {
	if m.CommunitiesResults != nil {
		return m.CommunitiesResults, nil
	}
	return []*query.StructuralCommunityResult{
		{
			ID:                   "comm-1",
			Name:                 "Community 1",
			Gamma:                0.05,
			Size:                 50,
			Density:              0.35,
			InternalEdgeCount:    120,
			BPRAvg:               0.08,
			SharedBoundaryCount:  2,
			CrossCuttingHubCount: 1,
			DominantDomains:      []string{"Billing", "Orders"},
		},
	}, nil
}

func (m *MockProvider) ClearStructuralTopology() error {
	m.ClearStructuralTopologyCalled = true
	return nil
}

func (m *MockProvider) UpdateStructuralTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	m.UpdateStructuralTopologyCalled = true
	return nil
}

func (m *MockProvider) GetCoverage(nodeID string) ([]*graph.Node, error) { return nil, nil }
func (m *MockProvider) LinkTests() error { return nil }

func (m *MockProvider) SeedVolatility(modulePattern string, rules []query.ContaminationRule) error {
	return nil
}
func (m *MockProvider) PropagateVolatility() error { return nil }
func (m *MockProvider) CalculateRiskScores() error { return nil }
func (m *MockProvider) CountVolatileFunctions() (int64, error) { return 0, nil }
func (m *MockProvider) HasVolatilityData() (bool, error)       { return true, nil }
func (m *MockProvider) UpdateFileHistory(metrics map[string]query.FileHistoryMetrics) error {
	return nil
}

func (m *MockProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error)    { return nil, nil }
func (m *MockProvider) CountUnextractedFunctions() (int64, error)                   { return 0, nil }
func (m *MockProvider) UpdateAtomicFeatures(id string, features []string, isVolatile bool) error {
	return nil
}
func (m *MockProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error)      { return nil, nil }
func (m *MockProvider) CountUnembeddedNodes() (int64, error)                     { return 0, nil }
func (m *MockProvider) UpdateEmbeddings(id string, embedding []float32) error    { return nil }
func (m *MockProvider) GetEmbeddingsOnly() (map[string][]float32, error)         { return nil, nil }
func (m *MockProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error)      { return nil, nil }
func (m *MockProvider) CountUnnamedFeatures() (int64, error)                     { return 0, nil }
func (m *MockProvider) ClearFeatureTopology() error                              { return nil }
func (m *MockProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	return nil
}
func (m *MockProvider) UpdateFeatureSummary(id string, name string, description string) error { return nil }
func (m *MockProvider) GetFunctionMetadata() ([]*graph.Node, error)                       { return nil, nil }

// MockLoader for testing/dry-run
type MockLoader struct{}

func (m *MockLoader) BatchLoadNodes(ctx context.Context, nodes []graph.Node) error { return nil }
func (m *MockLoader) BatchLoadEdges(ctx context.Context, edges []graph.Edge) error { return nil }
func (m *MockLoader) ApplyConstraints(ctx context.Context) error                   { return nil }
func (m *MockLoader) UpdateGraphState(ctx context.Context, commit string, dir string) error {
	return nil
}
func (m *MockLoader) WipeDatabase(ctx context.Context) error { return nil }


func (m *MockProvider) CreateBatchJobNode(ctx context.Context, jobID, modelName, gcsInput, gcsOutput string) error {
	return nil
}

func (m *MockProvider) UpdateBatchJobNodeStatus(ctx context.Context, jobID, state, failureReason string) error {
	return nil
}

func (m *MockProvider) GetActiveBatchJobs(ctx context.Context) ([]query.BatchJob, error) {
	return m.ActiveBatchJobs, nil
}

func (m *MockProvider) GetBatchJobCount(ctx context.Context) (int, error) {
	return m.BatchJobCount, nil
}

