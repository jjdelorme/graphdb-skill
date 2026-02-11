package analysis_test

import (
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

	content := []byte(`package com.example;

public class Sample {
    private int value;

    public Sample(int value) {
        this.value = value;
    }

    public void process() {
        helper();
    }

    private void helper() {
        System.out.println("Processing: " + value);
    }
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundSample := false
	foundProcess := false
	foundHelper := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "Sample" && n.Label == "Class" {
			foundSample = true
		}
		if name == "process" && n.Label == "Function" {
			foundProcess = true
		}
		if name == "helper" && n.Label == "Function" {
			foundHelper = true
		}
	}

	if !foundSample {
		t.Errorf("Expected Class 'Sample' not found")
	}
	if !foundProcess {
		t.Errorf("Expected Function 'process' not found")
	}
	if !foundHelper {
		t.Errorf("Expected Function 'helper' not found")
	}

	// Verify Call Edge from process -> helper
	foundCall := false
	for _, e := range edges {
		// Source: ...:process, Target: ...:helper
		// Note: The IDs depend on how the parser constructs them. Usually file path + name + line/col or similar.
		// We'll check for suffix for robustness.
		if strings.HasSuffix(e.SourceID, ":process") && strings.HasSuffix(e.TargetID, ":helper") && e.Type == "CALLS" {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge from process to helper not found")
	}
}
