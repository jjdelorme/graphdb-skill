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

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path   string
		isTest bool
	}{
		{"internal/analysis/parser_test.go", true},
		{"internal/analysis/parser.go", false},
		{"src/MyServiceTest.java", true},
		{"src/MyService.java", false},
		{"src/MyServiceTests.cs", true},
		{"src/MyService.cs", false},
		{"src/app.test.ts", true},
		{"src/app.spec.ts", true},
		{"src/app.ts", false},
		{"tests/integration/test_admin_endpoints_r2.py", true},
		{"test_admin_endpoints_r2.py", true},
		{"my_module_test.py", true},
		{"src/test/helpers.py", true},
		{"test/foo.py", true},
	}

	for _, tt := range tests {
		if got := IsTestFile(tt.path); got != tt.isTest {
			t.Errorf("IsTestFile(%q) = %v; want %v", tt.path, got, tt.isTest)
		}
	}
}
