package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

// Mocks
type MockDiscoverer struct{}

func (m *MockDiscoverer) DiscoverDomains(fileTree string) (map[string]string, error) {
	return map[string]string{
		"Auth":    "src/auth",
		"Payment": "src/payment",
	}, nil
}

type MockClusterer struct{}

func (m *MockClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	// Simple mock: Assign all nodes to a single cluster named after the domain + "Core"
	clusters := make(map[string][]graph.Node)
	clusters[domain+"Core"] = nodes
	return clusters, nil
}

func TestBuilder_Build(t *testing.T) {
	// Setup
	builder := &Builder{
		Discoverer: &MockDiscoverer{},
		Clusterer:  &MockClusterer{},
	}

	// Input: A mix of functions
	functions := []graph.Node{
		{ID: "func1", Properties: map[string]interface{}{"file_path": "src/auth/login.go"}},
		{ID: "func2", Properties: map[string]interface{}{"file_path": "src/payment/charge.go"}},
	}

	// Execute
	features, edges, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nodes, allEdges := Flatten(features, edges)

	// Verify Structure
	if len(features) != 2 {
		t.Errorf("Expected 2 domain features, got %d", len(features))
	}

	// Verify Nodes (Features should be included)
	// 2 Domains + 2 Clusters = 4 Feature nodes
	if len(nodes) != 4 {
		t.Errorf("Expected 4 feature nodes, got %d", len(nodes))
	}

	// Verify Edges
	// 2 PARENT_OF (Domain -> Cluster)
	// 2 IMPLEMENTS (Cluster -> Function)
	if len(allEdges) != 4 {
		t.Errorf("Expected 4 edges, got %d", len(allEdges))
	}

	foundImplements := 0
	foundParentOf := 0
	for _, e := range edges {
		if e.Type == "IMPLEMENTS" {
			foundImplements++
		}
		if e.Type == "PARENT_OF" {
			foundParentOf++
		}
	}

	if foundImplements != 2 {
		t.Errorf("Expected 2 IMPLEMENTS edges, got %d", foundImplements)
	}
	if foundParentOf != 2 {
		t.Errorf("Expected 2 PARENT_OF edges, got %d", foundParentOf)
	}
}
