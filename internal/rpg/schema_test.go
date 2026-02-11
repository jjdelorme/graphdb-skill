package rpg

import (
	"reflect"
	"testing"

	"graphdb/internal/graph"
)

func TestFeature_ToNode(t *testing.T) {
	embedding := []float32{0.1, 0.2, 0.3}
	feature := Feature{
		ID:          "feat-123",
		Name:        "Authentication",
		Description: "Handles user login and token generation",
		Embedding:   embedding,
		ScopePath:   "src/auth",
	}

	var node graph.Node = feature.ToNode()

	if node.ID != "feat-123" {
		t.Errorf("expected ID 'feat-123', got '%s'", node.ID)
	}
	if node.Label != "Feature" {
		t.Errorf("expected Label 'Feature', got '%s'", node.Label)
	}

	props := node.Properties
	if props["name"] != "Authentication" {
		t.Errorf("expected property 'name' to be 'Authentication', got '%v'", props["name"])
	}
	if props["description"] != "Handles user login and token generation" {
		t.Errorf("expected property 'description' to be correct, got '%v'", props["description"])
	}
	if props["scope_path"] != "src/auth" {
		t.Errorf("expected property 'scope_path' to be 'src/auth', got '%v'", props["scope_path"])
	}
	
	// Check embedding
	emb, ok := props["embedding"].([]float32)
	if !ok {
		t.Fatalf("expected property 'embedding' to be []float32")
	}
	if !reflect.DeepEqual(emb, embedding) {
		t.Errorf("expected embedding %v, got %v", embedding, emb)
	}
}
