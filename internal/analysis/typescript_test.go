package analysis_test

import (
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

	content := []byte(`function hello(name: string): void {
    console.log("Hello, " + name);
}
class Greeter {
    greet() { return "Hi"; }
}

function main() {
    hello("world");
    const g = new Greeter();
    g.greet();
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundHello := false
	foundGreeter := false
	foundGreet := false
	foundMain := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "hello" && n.Label == "Function" {
			foundHello = true
		}
		if name == "Greeter" && n.Label == "Class" {
			foundGreeter = true
		}
		if name == "greet" && n.Label == "Function" {
			foundGreet = true
		}
		if name == "main" && n.Label == "Function" {
			foundMain = true
		}
	}

	if !foundHello {
		t.Errorf("Expected Function 'hello' not found")
	}
	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Method 'greet' not found")
	}
	if !foundMain {
		t.Errorf("Expected Function 'main' not found")
	}

	// Helper to find edge
	hasEdge := func(srcName, tgtName string) bool {
		for _, e := range edges {
			// Check if SourceID ends with srcName and TargetID ends with tgtName
            // Note: srcName/tgtName passed here are simple names like "main", "hello"
            // The IDs are "path:name".
			if strings.HasSuffix(e.SourceID, ":"+srcName) && strings.HasSuffix(e.TargetID, ":"+tgtName) {
				return true
			}
		}
		return false
	}

	if !hasEdge("main", "hello") {
		t.Errorf("Expected Call Edge main -> hello not found")
	}
	if !hasEdge("main", "Greeter") {
		t.Errorf("Expected Call Edge main -> Greeter not found")
	}
	if !hasEdge("main", "greet") {
		t.Errorf("Expected Call Edge main -> greet not found")
	}
}
