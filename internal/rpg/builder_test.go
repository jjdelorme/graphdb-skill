package rpg

import (
	"graphdb/internal/graph"
	"strings"
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
		{ID: "func1", Properties: map[string]interface{}{"file": "src/auth/login.go"}},
		{ID: "func2", Properties: map[string]interface{}{"file": "src/payment/charge.go"}},
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
	// 2 IMPLEMENTS (Function -> Feature)
	if len(allEdges) != 4 {
		t.Errorf("Expected 4 edges, got %d", len(allEdges))
	}

	foundImplements := 0
	foundParentOf := 0
	for _, e := range edges {
		if e.Type == "IMPLEMENTS" {
			foundImplements++
			// Verify direction: SourceID should be the function, TargetID should be the feature
			if e.SourceID != "func1" && e.SourceID != "func2" {
				t.Errorf("IMPLEMENTS edge SourceID should be a function ID, got %s", e.SourceID)
			}
			if !strings.HasPrefix(e.TargetID, "feat-") {
				t.Errorf("IMPLEMENTS edge TargetID should be a feature ID (feat-*), got %s", e.TargetID)
			}
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

// MockCategoryClusterer splits nodes into two categories for testing
type MockCategoryClusterer struct{}

func (m *MockCategoryClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	clusters := make(map[string][]graph.Node)
	clusters[domain+"-cat"] = nodes
	return clusters, nil
}

func TestBuilder_BuildThreeLevel(t *testing.T) {
	builder := &Builder{
		Discoverer:        &MockDiscoverer{},
		CategoryClusterer: &MockCategoryClusterer{},
		Clusterer:         &MockClusterer{},
	}

	functions := []graph.Node{
		{ID: "func1", Properties: map[string]interface{}{"file": "src/auth/login.go"}},
		{ID: "func2", Properties: map[string]interface{}{"file": "src/payment/charge.go"}},
	}

	features, edges, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nodes, _ := Flatten(features, edges)

	// 2 domains + 2 categories + 2 features = 6 nodes
	if len(nodes) != 6 {
		t.Errorf("Expected 6 feature nodes in 3-level hierarchy, got %d", len(nodes))
	}

	// Verify edge types
	foundParentOf := 0
	foundImplements := 0
	for _, e := range edges {
		switch e.Type {
		case "PARENT_OF":
			foundParentOf++
		case "IMPLEMENTS":
			foundImplements++
		}
	}

	// 2 Domain->Category + 2 Category->Feature = 4 PARENT_OF
	if foundParentOf != 4 {
		t.Errorf("Expected 4 PARENT_OF edges in 3-level hierarchy, got %d", foundParentOf)
	}
	if foundImplements != 2 {
		t.Errorf("Expected 2 IMPLEMENTS edges, got %d", foundImplements)
	}

	// Verify hierarchy depth: domain -> category -> feature
	for _, domain := range features {
		if len(domain.Children) == 0 {
			continue
		}
		for _, cat := range domain.Children {
			if !strings.HasPrefix(cat.ID, "cat-") {
				t.Errorf("Expected category ID to start with 'cat-', got %s", cat.ID)
			}
			for _, feat := range cat.Children {
				if !strings.HasPrefix(feat.ID, "feat-") {
					t.Errorf("Expected feature ID to start with 'feat-', got %s", feat.ID)
				}
			}
		}
	}
}
