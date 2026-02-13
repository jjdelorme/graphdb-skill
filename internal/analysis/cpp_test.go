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

func TestParseCPP_Resolution(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/cpp/sample.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	content := []byte(`#include "math.h"
#include <iostream>

int global_counter = 0;

class Base {
public:
    int id;
};

class Derived : public Base {
public:
    void doWork() {
        global_counter++;
        id = 100;
        int result = Math::Add(5, 10);
    }
};

void main() {
    Derived d;
    d.doWork();
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 1. Check Inheritance
	hasInheritance := false
	for _, e := range edges {
		if strings.HasSuffix(e.SourceID, ":Derived") && strings.HasSuffix(e.TargetID, ":Base") && e.Type == "INHERITS" {
			hasInheritance = true
			break
		}
	}
	if !hasInheritance {
		t.Errorf("Expected INHERITS edge Derived -> Base")
	}

	// 2. Check Global
	hasGlobal := false
	for _, n := range nodes {
		if n.Label == "Global" && n.Properties["name"] == "global_counter" {
			hasGlobal = true
			break
		}
	}
	if !hasGlobal {
		t.Errorf("Expected Global node 'global_counter'")
	}

	// 3. Check Field
	hasField := false
	for _, n := range nodes {
		if n.Label == "Field" && n.Properties["name"] == "id" {
			hasField = true
			break
		}
	}
	if !hasField {
		t.Errorf("Expected Field node 'id'")
	}

	// 4. Check Usage of Global
	hasGlobalUsage := false
	for _, e := range edges {
		if strings.HasSuffix(e.SourceID, ":doWork") && strings.HasSuffix(e.TargetID, ":global_counter") && e.Type == "USES" {
			hasGlobalUsage = true
			break
		}
	}
	if !hasGlobalUsage {
		t.Errorf("Expected USES edge doWork -> global_counter")
	}

	// 5. Check Include Resolution (Math::Add -> math.h)
	// We expect Math::Add to resolve to something related to math.h
	// TargetID should contain "math.h"
	hasIncludeRes := false
	for _, e := range edges {
		// Searching for the call to Math::Add
		// The Source is doWork
		if strings.HasSuffix(e.SourceID, ":doWork") {
			// Check if TargetID points to math.h
			// The heuristic might map "Math" to "math.h"
			if strings.Contains(e.TargetID, "math.h") && strings.Contains(e.TargetID, "Math") {
				hasIncludeRes = true
				break
			}
		}
	}
	if !hasIncludeRes {
		t.Errorf("Expected resolution of Math::Add to math.h, but didn't find edge")
	}
}
