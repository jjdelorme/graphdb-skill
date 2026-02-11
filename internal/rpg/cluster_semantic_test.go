package rpg

import (
	"graphdb/internal/graph"
	"math"
	"testing"
)

// deterministicEmbedder returns embeddings that cluster into known groups.
type deterministicEmbedder struct{}

func (d *deterministicEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	// Map specific texts to known embedding regions
	res := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, 4)
		switch {
		case t == "validate input, check credentials":
			vec = []float32{1, 0, 0, 0}
		case t == "verify token, authenticate user":
			vec = []float32{0.9, 0.1, 0, 0}
		case t == "hash password, encrypt data":
			vec = []float32{0.8, 0.2, 0, 0}
		case t == "write file, save data":
			vec = []float32{0, 0, 1, 0}
		case t == "read file, load config":
			vec = []float32{0, 0, 0.9, 0.1}
		case t == "serialize data, format output":
			vec = []float32{0, 0, 0.8, 0.2}
		default:
			vec = []float32{0.5, 0.5, 0.5, 0.5}
		}
		res[i] = vec
	}
	return res, nil
}

func TestEmbeddingClusterer_Cluster(t *testing.T) {
	clusterer := &EmbeddingClusterer{
		Embedder:      &deterministicEmbedder{},
		MaxIterations: 100,
	}

	nodes := []graph.Node{
		{ID: "fn1", Properties: map[string]interface{}{
			"name":            "validateInput",
			"atomic_features": []string{"validate input", "check credentials"},
		}},
		{ID: "fn2", Properties: map[string]interface{}{
			"name":            "verifyToken",
			"atomic_features": []string{"verify token", "authenticate user"},
		}},
		{ID: "fn3", Properties: map[string]interface{}{
			"name":            "hashPassword",
			"atomic_features": []string{"hash password", "encrypt data"},
		}},
		{ID: "fn4", Properties: map[string]interface{}{
			"name":            "writeFile",
			"atomic_features": []string{"write file", "save data"},
		}},
		{ID: "fn5", Properties: map[string]interface{}{
			"name":            "readFile",
			"atomic_features": []string{"read file", "load config"},
		}},
		{ID: "fn6", Properties: map[string]interface{}{
			"name":            "serializeData",
			"atomic_features": []string{"serialize data", "format output"},
		}},
	}

	clusters, err := clusterer.Cluster(nodes, "auth")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	// With 6 nodes and target of 3-8 per cluster, we expect ~2 clusters
	if len(clusters) < 2 {
		t.Errorf("Expected at least 2 clusters, got %d", len(clusters))
	}

	// Verify all nodes are assigned
	total := 0
	for _, fns := range clusters {
		total += len(fns)
	}
	if total != 6 {
		t.Errorf("Expected 6 total nodes across clusters, got %d", total)
	}

	t.Logf("Clusters: %d", len(clusters))
	for name, fns := range clusters {
		ids := make([]string, len(fns))
		for i, fn := range fns {
			ids[i] = fn.ID
		}
		t.Logf("  %s: %v", name, ids)
	}
}

func TestEmbeddingClusterer_SmallInput(t *testing.T) {
	clusterer := &EmbeddingClusterer{
		Embedder: &deterministicEmbedder{},
	}

	nodes := []graph.Node{
		{ID: "fn1", Properties: map[string]interface{}{"name": "a"}},
		{ID: "fn2", Properties: map[string]interface{}{"name": "b"}},
	}

	clusters, err := clusterer.Cluster(nodes, "small")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	// With <= 3 nodes, should get a single cluster
	if len(clusters) != 1 {
		t.Errorf("Expected 1 cluster for small input, got %d", len(clusters))
	}
}

func TestEmbeddingClusterer_Empty(t *testing.T) {
	clusterer := &EmbeddingClusterer{
		Embedder: &deterministicEmbedder{},
	}

	clusters, err := clusterer.Cluster(nil, "empty")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	if clusters != nil {
		t.Errorf("Expected nil for empty input, got %v", clusters)
	}
}

func TestCosineDistance(t *testing.T) {
	// Same direction should be distance ~0
	a := []float32{1, 0, 0}
	b := []float32{2, 0, 0}
	d := cosineDistance(a, b)
	if d > 0.001 {
		t.Errorf("Same-direction vectors should have near-zero distance, got %f", d)
	}

	// Orthogonal vectors should have distance ~1
	c := []float32{0, 1, 0}
	d2 := cosineDistance(a, c)
	if math.Abs(float64(d2)-1.0) > 0.001 {
		t.Errorf("Orthogonal vectors should have distance ~1, got %f", d2)
	}
}

func TestKmeans_Basic(t *testing.T) {
	// Two clear clusters
	vectors := [][]float32{
		{1, 0},
		{0.9, 0.1},
		{0, 1},
		{0.1, 0.9},
	}
	assignments := kmeans(vectors, 2, 50)

	// First two should be in one cluster, last two in another
	if assignments[0] != assignments[1] {
		t.Error("Expected vectors 0 and 1 to be in same cluster")
	}
	if assignments[2] != assignments[3] {
		t.Error("Expected vectors 2 and 3 to be in same cluster")
	}
	if assignments[0] == assignments[2] {
		t.Error("Expected vectors 0 and 2 to be in different clusters")
	}
}
