package analysis

import (
	"testing"

	"graphdb/internal/graph"
)

// MockParser is a dummy parser for testing purposes.
type MockParser struct{}

func (m *MockParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	// Return dummy data
	nodes := []*graph.Node{
		{ID: "node1", Label: "Function", Properties: map[string]interface{}{"name": "test"}},
	}
	edges := []*graph.Edge{
		{SourceID: "node1", TargetID: "node2", Type: "CALLS"},
	}
	return nodes, edges, nil
}

func TestGetParser(t *testing.T) {
	// 1. Register a mock parser
	mock := &MockParser{}
	RegisterParser(".mock", mock)

	// 2. Retrieve it
	p, ok := GetParser(".mock")
	if !ok {
		t.Fatalf("Expected to find parser for .mock")
	}

	if p != mock {
		t.Errorf("Expected retrieved parser to be the same instance as registered")
	}

	// 3. Try retrieving a non-existent parser
	_, ok = GetParser(".nonexistent")
	if ok {
		t.Errorf("Expected not to find parser for .nonexistent")
	}
}

func TestParse(t *testing.T) {
	mock := &MockParser{}
	
	nodes, edges, err := mock.Parse("test.mock", []byte("content"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edges))
	}
}
