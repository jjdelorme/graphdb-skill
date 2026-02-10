package storage_test

import (
	"bytes"
	"encoding/json"
	"graphdb/internal/graph"
	"graphdb/internal/storage"
	"testing"
)

func TestJSONLEmitter_EmitNode(t *testing.T) {
	var buf bytes.Buffer
	emitter := storage.NewJSONLEmitter(&buf)

	node := &graph.Node{
		ID:    "node-1",
		Label: "Function",
		Properties: map[string]interface{}{
			"name":  "testFunc",
			"lines": 50,
		},
	}

	if err := emitter.EmitNode(node); err != nil {
		t.Fatalf("EmitNode failed: %v", err)
	}

	// Read back and verify
	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	if output["id"] != "node-1" {
		t.Errorf("Expected id 'node-1', got %v", output["id"])
	}
	// Verify mapping from Label -> type
	if output["type"] != "Function" {
		t.Errorf("Expected type 'Function', got %v", output["type"])
	}
	// Verify property flattening
	if output["name"] != "testFunc" {
		t.Errorf("Expected name 'testFunc', got %v", output["name"])
	}
	// JSON unmarshals numbers as float64
	if output["lines"] != 50.0 {
		t.Errorf("Expected lines 50, got %v", output["lines"])
	}
}

func TestJSONLEmitter_EmitEdge(t *testing.T) {
	var buf bytes.Buffer
	emitter := storage.NewJSONLEmitter(&buf)

	edge := &graph.Edge{
		SourceID: "node-1",
		TargetID: "node-2",
		Type:     "CALLS",
	}

	if err := emitter.EmitEdge(edge); err != nil {
		t.Fatalf("EmitEdge failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	// Verify mappings
	if output["source"] != "node-1" {
		t.Errorf("Expected source 'node-1', got %v", output["source"])
	}
	if output["target"] != "node-2" {
		t.Errorf("Expected target 'node-2', got %v", output["target"])
	}
	if output["type"] != "CALLS" {
		t.Errorf("Expected type 'CALLS', got %v", output["type"])
	}
}
