package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseCPP(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/cpp/sample.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`#include <iostream>
void hello() {
    std::cout << "Hello";
}
class Greeter {
public:
    void greet() { hello(); }
};`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundHello := false
	foundGreet := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "hello" && n.Label == "Function" {
			foundHello = true
		}
		if name == "greet" && n.Label == "Function" {
			foundGreet = true
		}
	}

	if !foundHello {
		t.Errorf("Expected Function 'hello' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'greet' not found")
	}

	// Helper to find edge
	hasEdge := func(srcName, tgtName string) bool {
		for _, e := range edges {
			if strings.HasSuffix(e.SourceID, ":"+srcName) && strings.HasSuffix(e.TargetID, ":"+tgtName) {
				return true
			}
		}
		return false
	}

	if !hasEdge("greet", "hello") {
		t.Errorf("Expected Call Edge greet -> hello not found")
	}
}
