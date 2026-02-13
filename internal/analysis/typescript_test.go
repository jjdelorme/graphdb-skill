package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseTypeScript(t *testing.T) {
	parser, ok := analysis.GetParser(".ts")
	if !ok {
		t.Fatalf("TypeScript parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/typescript/sample.ts")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Read content from file instead of hardcoded string
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
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
	
	// Helper to find specific edge type
	hasEdgeType := func(srcName, tgtName, edgeType string) bool {
		for _, e := range edges {
			if strings.HasSuffix(e.SourceID, ":"+srcName) && 
			   strings.HasSuffix(e.TargetID, ":"+tgtName) && 
			   e.Type == edgeType {
				return true
			}
		}
		return false
	}

	// Basic checks
	foundHello := false
	foundGreeter := false
	
	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "hello" && n.Label == "Function" {
			foundHello = true
		}
		if name == "Greeter" && n.Label == "Class" {
			foundGreeter = true
		}
	}

	if !foundHello {
		t.Errorf("Expected Function 'hello' not found")
	}
	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}

	if !hasEdge("main", "hello") {
		t.Errorf("Expected Call Edge main -> hello not found")
	}
	
	// 1. Check for Import Resolution
	foundUserUsage := false
	for _, e := range edges {
		if strings.HasSuffix(e.SourceID, ":main") && strings.Contains(e.TargetID, "models/User.ts:User") {
			foundUserUsage = true
			break
		}
	}
	if !foundUserUsage {
		t.Errorf("Expected Call Edge main -> models/User.ts:User not found")
	}

	// 2. Check for Extends
	if !hasEdgeType("SuperUser", "User", "EXTENDS") {
		// Note: The target ID for extends should also be resolved to models/User.ts:User
        // My hasEdgeType helper checks suffix ":User", which is fine as long as ID ends with it.
        // But let's be strict:
        foundExtends := false
        for _, e := range edges {
            if strings.HasSuffix(e.SourceID, ":SuperUser") && 
               strings.Contains(e.TargetID, "models/User.ts:User") && 
               e.Type == "EXTENDS" {
                foundExtends = true
                break
            }
        }
        if !foundExtends {
             t.Errorf("Expected EXTENDS Edge SuperUser -> models/User.ts:User not found")
        }
	}

    // 3. Check for Properties
    foundRole := false
    for _, n := range nodes {
        name, _ := n.Properties["name"].(string)
        if name == "role" && n.Label == "Field" {
            foundRole = true
        }
    }
    if !foundRole {
        t.Errorf("Expected Field 'role' not found")
    }

    // 4. Check for Alias Resolution
    // UserAlias -> User
    // Usage in main: const u2 = new UserAlias(...)
    // Should create edge main -> models/User.ts:User
    // Since we already found one usage (foundUserUsage), we need to ensure we have TWO calls?
    // Or just that logic works.
    // Let's verify that we don't have an edge to "UserAlias".
    
    foundAliasEdge := false
    for _, e := range edges {
        if strings.HasSuffix(e.TargetID, ":UserAlias") {
            foundAliasEdge = true
            break
        }
    }
    if foundAliasEdge {
        t.Errorf("Found edge to alias 'UserAlias', expected resolution to 'User'")
    }
}
