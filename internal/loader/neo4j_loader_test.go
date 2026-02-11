package loader

import (
	"graphdb/internal/graph"
	"strings"
	"testing"
)

func TestBuildNodeQuery(t *testing.T) {
	query := buildNodeQuery("Function")
	if !strings.Contains(query, "UNWIND $batch AS row") {
		t.Error("Missing UNWIND clause")
	}
	if !strings.Contains(query, "MERGE (n:Function {id: row.id})") {
		t.Error("Missing MERGE clause with correct label")
	}
}

func TestBuildEdgeQuery(t *testing.T) {
	query := buildEdgeQuery("CALLS")
	if !strings.Contains(query, "UNWIND $batch AS row") {
		t.Error("Missing UNWIND clause")
	}
	if !strings.Contains(query, "MERGE (source)-[r:CALLS]->(target)") {
		t.Error("Missing MERGE clause with correct type")
	}
}

func TestGroupNodesByLabel(t *testing.T) {
	nodes := []graph.Node{
		{ID: "1", Label: "Function", Properties: map[string]any{"name": "a"}},
		{ID: "2", Label: "Function", Properties: map[string]any{"name": "b"}},
		{ID: "3", Label: "Class", Properties: map[string]any{"name": "c"}},
	}
	
	batches := groupNodesByLabel(nodes)
	if len(batches) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(batches))
	}
	if len(batches["Function"]) != 2 {
		t.Errorf("Expected 2 Functions, got %d", len(batches["Function"]))
	}
	if len(batches["Class"]) != 1 {
		t.Errorf("Expected 1 Class, got %d", len(batches["Class"]))
	}
}

func TestBuildWipeQuery(t *testing.T) {
	query := buildWipeQuery()
	if !strings.Contains(query, "MATCH (n) DETACH DELETE n") {
		t.Error("Missing DETACH DELETE clause")
	}
}

func TestBuildGraphStateQuery(t *testing.T) {
	query := buildGraphStateQuery()
	if !strings.Contains(query, "MERGE (s:GraphState)") {
		t.Error("Missing GraphState node merge")
	}
	if !strings.Contains(query, "SET s.commit = $commit") {
		t.Error("Missing commit set")
	}
}

