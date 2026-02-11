package e2e_test

import (
	"context"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/ingest"
	"testing"
    "path/filepath"
    "os"
)

// MockEmbedder implements embedding.Embedder
type MockEmbedder struct{}

var _ embedding.Embedder = (*MockEmbedder)(nil)

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3} // Dummy embedding
	}
	return result, nil
}

// MockEmitter implements storage.Emitter
type MockEmitter struct {
	Nodes []*graph.Node
	Edges []*graph.Edge
}

func (m *MockEmitter) EmitNode(node *graph.Node) error {
	m.Nodes = append(m.Nodes, node)
	return nil
}

func (m *MockEmitter) EmitEdge(edge *graph.Edge) error {
	m.Edges = append(m.Edges, edge)
	return nil
}

func (m *MockEmitter) Close() error {
	return nil
}

func TestWalker_Run(t *testing.T) {
    // Determine the absolute path to fixtures
    wd, err := os.Getwd()
    if err != nil {
        t.Fatal(err)
    }
    // We are likely running from root or test/e2e. 
    // If running from root via `go test ./test/e2e/...`, wd is root.
    // Let's assume the test is running from the module root.
    fixturesPath := filepath.Join(wd, "../../test/fixtures/typescript")
    
    // Check if path exists, if not, try to find it relative to where we are
    if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
         // Maybe we are in test/e2e?
         fixturesPath = filepath.Join(wd, "../fixtures/typescript")
         if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
             // Maybe we are in root?
             fixturesPath = filepath.Join(wd, "test/fixtures/typescript")
         }
    }

	emitter := &MockEmitter{}
	embedder := &MockEmbedder{}

    walker := ingest.NewWalker(2, embedder, emitter)

    err = walker.Run(context.Background(), fixturesPath)
    if err != nil {
        t.Fatalf("Walker.Run failed: %v", err)
    }

    // Since the Walker/Worker logic is stubbed, we expect 0 nodes for now (or validation failure if implemented).
    // But this is TDD, so we expect the test to FAIL or pass trivially if we assert 0.
    // The goal is to assert that we HAVE nodes. So this test should FAIL until we implement the logic.
    
    if len(emitter.Nodes) == 0 {
        t.Errorf("Expected nodes to be emitted, got 0. Path: %s", fixturesPath)
    }
}
