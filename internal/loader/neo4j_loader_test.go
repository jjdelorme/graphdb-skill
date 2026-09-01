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
	if !strings.Contains(query, "MERGE (n:CodeElement {id: row.id})") {
		t.Error("Missing MERGE clause with correct label")
	}
	if !strings.Contains(query, "SET n:Function") {
		t.Error("Missing SET n:Function clause")
	}
}

func TestBuildEdgeQuery(t *testing.T) {
	query := buildEdgeQuery("CALLS")
	if !strings.Contains(query, "UNWIND $batch AS row") {
		t.Error("Missing UNWIND clause")
	}
	if !strings.Contains(query, "MATCH (source:CodeElement {id: row.sourceId})") {
		t.Error("Missing MATCH source with :CodeElement label")
	}
	if !strings.Contains(query, "MATCH (target:CodeElement) WHERE target.id = row.targetId OR target.fqn = row.targetId") {
		t.Error("Missing MATCH target with :CodeElement label and polymorphic matching")
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

func TestBuildGraphStateQuery(t *testing.T) {
	// Test Global
	query := buildGraphStateQuery(false)
	if !strings.Contains(query, "MERGE (s:GraphState)") {
		t.Error("Missing GraphState node merge")
	}
	if !strings.Contains(query, "SET s.commit = $commit") {
		t.Error("Missing commit set")
	}

	// Test Dir-specific
	queryDir := buildGraphStateQuery(true)
	if !strings.Contains(queryDir, "MERGE (s:Metadata {dir: $dir})") {
		t.Error("Missing Metadata node merge with dir")
	}
}

func TestGetConstraints(t *testing.T) {
	// Test Default (768)
	l := NewNeo4jLoader(nil, "test", 768)
	constraints := l.getConstraints()
	
	hasFeatureVectorIndex := false
	hasFunctionVectorIndex := false
	hasStructuralCommConstraint := false
	hasStructuralCommNameIndex := false
	hasStructuralCommSizeIndex := false
	hasSharedBoundaryIndex := false
	hasCrossCuttingHubIndex := false
	
	for _, q := range constraints {
		if strings.Contains(q, "CREATE CONSTRAINT IF NOT EXISTS FOR (c:StructuralCommunity) REQUIRE c.id IS UNIQUE") {
			hasStructuralCommConstraint = true
		}
		if strings.Contains(q, "CREATE INDEX IF NOT EXISTS FOR (c:StructuralCommunity) ON (c.name)") {
			hasStructuralCommNameIndex = true
		}
		if strings.Contains(q, "CREATE INDEX IF NOT EXISTS FOR (c:StructuralCommunity) ON (c.size)") {
			hasStructuralCommSizeIndex = true
		}
		if strings.Contains(q, "CREATE INDEX IF NOT EXISTS FOR (n:SharedBoundary) ON (n.id)") {
			hasSharedBoundaryIndex = true
		}
		if strings.Contains(q, "CREATE INDEX IF NOT EXISTS FOR (n:CrossCuttingHub) ON (n.id)") {
			hasCrossCuttingHubIndex = true
		}
		if strings.Contains(q, "CREATE VECTOR INDEX feature_embeddings") {
			hasFeatureVectorIndex = true
			if !strings.Contains(q, "FOR (n:Feature) ON (n.embedding)") {
				t.Error("feature_embeddings index has wrong target")
			}
			if !strings.Contains(q, "768") {
				t.Error("feature_embeddings index has wrong dimensions (expected 768)")
			}
		}
		if strings.Contains(q, "CREATE VECTOR INDEX function_embeddings") {
			hasFunctionVectorIndex = true
			if !strings.Contains(q, "FOR (n:Function) ON (n.embedding)") {
				t.Error("function_embeddings index has wrong target")
			}
		}
	}
	
	if !hasStructuralCommConstraint {
		t.Error("Missing :StructuralCommunity(id) uniqueness constraint")
	}
	if !hasStructuralCommNameIndex {
		t.Error("Missing :StructuralCommunity(name) index")
	}
	if !hasStructuralCommSizeIndex {
		t.Error("Missing :StructuralCommunity(size) index")
	}
	if !hasSharedBoundaryIndex {
		t.Error("Missing :SharedBoundary(id) index")
	}
	if !hasCrossCuttingHubIndex {
		t.Error("Missing :CrossCuttingHub(id) index")
	}
	if !hasFeatureVectorIndex {
		t.Error("Missing feature_embeddings vector index")
	}
	if !hasFunctionVectorIndex {
		t.Error("Missing function_embeddings vector index")
	}

	// Test Dynamic Dimensions (3072)
	l2 := NewNeo4jLoader(nil, "test", 3072)
	constraints2 := l2.getConstraints()
	found3072 := false
	for _, q := range constraints2 {
		if strings.Contains(q, "3072") {
			found3072 = true
			break
		}
	}
	if !found3072 {
		t.Error("Dynamic dimensions not reflected in constraints")
	}
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Function", "Function"},
		{"`Function`", "Function"},
		{"My-Label", "MyLabel"},
		{"Label with space", "Labelwithspace"},
		{"Label;DROP TABLE", "LabelDROPTABLE"},
		{"Label_123", "Label_123"},
	}

	for _, tc := range tests {
		got := SanitizeLabel(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeLabel(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
