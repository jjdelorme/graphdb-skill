//go:build test_mocks

package main

import (
	"context"
	"graphdb/internal/config"
	"graphdb/internal/graph"
	"graphdb/internal/query"
	"os"
	"testing"
)

func TestHandleEnrichTopology_Mock(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	// Custom mock provider returning sample nodes and edges from RunCypher
	mockProvider := &MockProvider{}

	originalSetupProvider := setupProviderFn
	defer func() {
		setupProviderFn = originalSetupProvider
	}()

	setupProviderFn = func(cfg config.Config) (query.GraphProvider, error) {
		return mockProvider, nil
	}

	args := []string{"-dir", "test_project", "-gamma", "0.05", "-min-size", "2", "-max-size", "10", "-seed", "42", "--offline"}
	handleEnrichTopology(args)

	if !mockProvider.ClearStructuralTopologyCalled {
		t.Error("Expected ClearStructuralTopology to be called during enrich-topology")
	}

	if !mockProvider.UpdateStructuralTopologyCalled {
		t.Error("Expected UpdateStructuralTopology to be called during enrich-topology")
	}
}

func TestHandleEnrichTopology_WithNodesAndEdges(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687")
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	var persistedNodes []*graph.Node
	var persistedEdges []*graph.Edge

	testMock := &testCustomMockProvider{
		MockProvider: &MockProvider{},
		runCypherFn: func(q string) ([]map[string]any, error) {
			if q == `
		// Fetch CodeElement Nodes for Topology
		MATCH (n:CodeElement)
		RETURN n.id AS id
	` {
				return []map[string]any{
					{"id": "func1"},
					{"id": "func2"},
					{"id": "func3"},
				}, nil
			}
			return []map[string]any{
				{"source": "func1", "target": "func2", "type": "CALLS"},
				{"source": "func2", "target": "func3", "type": "CALLS"},
			}, nil
		},
		updateTopologyFn: func(nodes []*graph.Node, edges []*graph.Edge) error {
			persistedNodes = nodes
			persistedEdges = edges
			return nil
		},
	}

	originalSetupProvider := setupProviderFn
	defer func() {
		setupProviderFn = originalSetupProvider
	}()

	setupProviderFn = func(cfg config.Config) (query.GraphProvider, error) {
		return testMock, nil
	}

	args := []string{"-gamma", "0.01", "-seed", "123"}
	handleEnrichTopology(args)

	if len(persistedNodes) == 0 {
		t.Error("Expected persisted nodes to be non-empty")
	}

	foundCommunity := false
	for _, n := range persistedNodes {
		if n.Label == "StructuralCommunity" {
			foundCommunity = true
			break
		}
	}
	if !foundCommunity {
		t.Error("Expected at least one StructuralCommunity node persisted")
	}

	foundInCommEdge := false
	for _, e := range persistedEdges {
		if e.Type == "IN_COMMUNITY" {
			foundInCommEdge = true
			break
		}
	}
	if !foundInCommEdge {
		t.Error("Expected at least one IN_COMMUNITY edge persisted")
	}
}

type testCustomMockProvider struct {
	*MockProvider
	runCypherFn      func(query string) ([]map[string]any, error)
	updateTopologyFn func(nodes []*graph.Node, edges []*graph.Edge) error
}

func (t *testCustomMockProvider) RunCypher(query string) ([]map[string]any, error) {
	if t.runCypherFn != nil {
		return t.runCypherFn(query)
	}
	return t.MockProvider.RunCypher(query)
}

func (t *testCustomMockProvider) UpdateStructuralTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	_ = t.MockProvider.UpdateStructuralTopology(nodes, edges)
	if t.updateTopologyFn != nil {
		return t.updateTopologyFn(nodes, edges)
	}
	return nil
}
