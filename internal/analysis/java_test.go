package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseJava(t *testing.T) {
	parser, ok := analysis.GetParser(".java")
	if !ok {
		t.Skip("Java parser not registered (yet)")
	}

	absPath, err := filepath.Abs("../../test/fixtures/java/sample.java")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Helper to find node by name and label
	findNode := func(name, label string) bool {
		for _, n := range nodes {
			nName, _ := n.Properties["name"].(string)
			if nName == name && n.Label == label {
				return true
			}
		}
		return false
	}

	// 1. Verify Structure
	if !findNode("Sample", "Class") {
		t.Errorf("Expected Class 'Sample' not found")
	}
	if !findNode("Base", "Class") {
		t.Errorf("Expected Class 'Base' not found")
	}
	if !findNode("Worker", "Interface") {
		t.Errorf("Expected Interface 'Worker' not found")
	}
	if !findNode("items", "Field") {
		t.Errorf("Expected Field 'items' not found")
	}
	if !findNode("helper", "Field") {
		t.Errorf("Expected Field 'helper' not found")
	}

	// 2. Verify Inheritance / Implementation Edges
	foundExtends := false
	foundImplements := false

	for _, e := range edges {
		// We expect Sample -> Base (EXTENDS)
		if strings.Contains(e.SourceID, "Sample") && strings.Contains(e.TargetID, "Base") && e.Type == "EXTENDS" {
			foundExtends = true
		}
		// We expect Sample -> Worker (IMPLEMENTS)
		if strings.Contains(e.SourceID, "Sample") && strings.Contains(e.TargetID, "Worker") && e.Type == "IMPLEMENTS" {
			foundImplements = true
		}
	}

	if !foundExtends {
		t.Errorf("Expected EXTENDS edge from Sample to Base not found")
	}
	if !foundImplements {
		t.Errorf("Expected IMPLEMENTS edge from Sample to Worker not found")
	}

	// 3. Verify Import Resolution / Uses
    // Check call to helper.doWork() -> should link to Worker:doWork
    foundWorkerCall := false
    // Check call to items.size() -> should link to java.util.List:size
    foundListCall := false

    for _, e := range edges {
        if e.Type == "CALLS" {
            // Check TargetID contains "Worker:doWork" (ignoring full package prefix issues for now, just substring)
            if strings.Contains(e.TargetID, "Worker:doWork") {
                foundWorkerCall = true
            }
            if strings.Contains(e.TargetID, "java.util.List:size") {
                foundListCall = true
            }
        }
    }

    if !foundWorkerCall {
        t.Errorf("Expected CALLS edge to Worker:doWork not found")
    }
    if !foundListCall {
        t.Errorf("Expected CALLS edge to java.util.List:size not found")
    }
}
