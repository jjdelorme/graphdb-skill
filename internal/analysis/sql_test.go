package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseSQL(t *testing.T) {
	// 1. Setup
	absPath, err := filepath.Abs("../../test/fixtures/sql/sample.sql")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// 2. Get Parser
	parser, ok := analysis.GetParser(".sql")
	if !ok {
		t.Fatalf("SQL parser not registered")
	}

	// 3. Parse
	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 4. Verification
	foundCalculateTotal := false
	foundProcessOrder := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "CalculateTotal" && n.Label == "Function" {
			foundCalculateTotal = true
		}
		if name == "ProcessOrder" && n.Label == "Function" { // Procedure treated as Function
			foundProcessOrder = true
		}
	}

	if !foundCalculateTotal {
		t.Errorf("Expected Function 'CalculateTotal' not found")
	}
	if !foundProcessOrder {
		t.Errorf("Expected Function/Procedure 'ProcessOrder' not found")
	}

	// Helper to find edge
	hasEdge := func(srcName, tgtName string) bool {
		for _, e := range edges {
			// Source and Target IDs are typically "filepath:name"
			if strings.HasSuffix(e.SourceID, ":"+srcName) && strings.HasSuffix(e.TargetID, ":"+tgtName) {
				return true
			}
		}
		return false
	}

	if !hasEdge("ProcessOrder", "CalculateTotal") {
		t.Errorf("Expected Call Edge ProcessOrder -> CalculateTotal not found")
	}
}
