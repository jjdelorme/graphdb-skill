package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseCSharp(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/csharp/sample.cs")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`using System;
public class Greeter {
    public void Greet(string name) {
        Console.WriteLine("Hello " + name);
    }
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundGreet := false
	foundGreeter := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "Greet" && n.Label == "Function" {
			foundGreet = true
		}
		if name == "Greeter" && n.Label == "Class" {
			foundGreeter = true
		}
	}

	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'Greet' not found")
	}

	// Verify Call Edge
	foundCall := false
	for _, e := range edges {
		// Source: ...:Greet
		// Target: WriteLine OR System.WriteLine (Resolution candidates)
		// Old behavior was ...:WriteLine. New behavior is logical ID.
		if strings.HasSuffix(e.SourceID, ":Greet") && (strings.HasSuffix(e.TargetID, "WriteLine") || e.TargetID == "WriteLine") {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge from Greet to WriteLine not found")
	}
}
