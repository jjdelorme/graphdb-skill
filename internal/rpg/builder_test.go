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
	features, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify
	// We expect Top Level Features (Domains) -> Child Features (Clusters)
	if len(features) != 2 {
		t.Errorf("Expected 2 domain features, got %d", len(features))
	}

	// Check for Auth domain
	var authFeature *Feature
	for _, f := range features {
		if f.Name == "Auth" {
			authFeature = &f
			break
		}
	}

	if authFeature == nil {
		t.Fatal("Auth domain not found")
	}

	// We expect the MockClusterer to have attached a child feature "AuthCore"
	// Wait, the Builder needs to return a Tree structure or a flat list of nodes?
	// The return type of Build should probably be a list of *Feature, where Features have Children.
}
