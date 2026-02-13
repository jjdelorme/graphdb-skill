package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestCSharp_SystemicResolution(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/csharp/dependency.cs")
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

	// Helper to find node by partial ID
	findNode := func(partialID string) bool {
		for _, n := range nodes {
			if strings.Contains(n.ID, partialID) {
				return true
			}
		}
		return false
	}

    // Helper to find edge
    findEdge := func(sourceSub, targetSub, edgeType string) bool {
        for _, e := range edges {
            if strings.Contains(e.SourceID, sourceSub) && 
               strings.Contains(e.TargetID, targetSub) && 
               e.Type == edgeType {
                return true
            }
        }
        return false
    }

	// 1. Verify Namespace in ID (MyCorp.App.UserManager)
    // The current parser uses "filePath:Name". 
    // We want "filePath:MyCorp.App.UserManager" or similar logic if we switch to semantic IDs, 
    // but the plan says "Use Logical IDs ... Namespace.ClassName" or "File-Inferred IDs".
    // Let's assume we want to see the namespace in the node name or ID.
    // Current implementation: ID = "filePath:UserManager".
    // Desired: ID = "filePath:MyCorp.App.UserManager" (or just capturing the namespace property).
    // Let's check for "MyCorp.App.UserManager" in the ID.
	if !findNode("MyCorp.App.UserManager") {
		t.Errorf("Missing Namespaced Node: MyCorp.App.UserManager")
	}

	// 2. Verify Inheritance
	if !findEdge("UserManager", "BaseManager", "INHERITS") {
		t.Errorf("Missing INHERITS edge: UserManager -> BaseManager")
	}

    // 3. Verify Fields/Properties
    if !findNode("AppName") {
        t.Errorf("Missing Field/Property Node: AppName")
    }
    if !findNode("_logger") {
        t.Errorf("Missing Field Node: _logger")
    }

    // 4. Verify Resolution (List<string>)
    // Should resolve to System.Collections.Generic.List based on `using`.
    // The target ID should be something like "System.Collections.Generic.List" or at least "List" with context.
    // The plan says: Link to `TypeName` (unqualified) but add property `possible_namespaces`.
    // OR "Identify explicit matches".
    // Let's aim for the Edge TargetID to be "System.Collections.Generic.List" if we implement the resolution.
    if !findEdge("ProcessUsers", "System.Collections.Generic.List", "CALLS") {
         t.Errorf("Missing or Unresolved CALLS edge to System.Collections.Generic.List")
    }
}
