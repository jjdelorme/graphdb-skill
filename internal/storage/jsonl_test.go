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

func TestJSONLEmitter_Concurrent(t *testing.T) {
	// Concurrent writes to a bytes.Buffer are not safe, so we need a thread-safe writer
	// or rely on JSONLEmitter to serialize access.
	// Since we are testing JSONLEmitter's thread safety, we expect it to protect the writer.
	// However, bytes.Buffer itself is not thread safe, so if JSONLEmitter doesn't lock,
	// this test will likely panic or fail with -race.
	var buf bytes.Buffer
	emitter := storage.NewJSONLEmitter(&buf)

	concurrency := 10
	itemsPerRoutine := 100
	done := make(chan bool)

	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < itemsPerRoutine; j++ {
				_ = emitter.EmitNode(&graph.Node{
					ID:    "node",
					Label: "test",
				})
			}
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Basic check: we should have (concurrency * itemsPerRoutine) lines
	// Note: checking line count accurately requires splitting the buffer.
	// If race occurs, output might be garbled.
	lines := bytes.Count(buf.Bytes(), []byte("\n"))
	expected := concurrency * itemsPerRoutine
	if lines != expected {
		t.Errorf("Expected %d lines, got %d", expected, lines)
	}
}
