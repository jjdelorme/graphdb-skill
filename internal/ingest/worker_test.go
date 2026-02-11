package ingest

import (
	"errors"
	"graphdb/internal/graph"
	"testing"
	"sync"
	"path/filepath"
	"os"
)

// MockEmbedder always fails
type MockFailingEmbedder struct{}

func (m *MockFailingEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	return nil, errors.New("simulated embedding failure")
}

// MockEmitter collects items
type MockEmitter struct {
	Nodes []*graph.Node
	Edges []*graph.Edge
	mu    sync.Mutex
}

func (m *MockEmitter) EmitNode(node *graph.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Nodes = append(m.Nodes, node)
	return nil
}

func (m *MockEmitter) EmitEdge(edge *graph.Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Edges = append(m.Edges, edge)
	return nil
}

func (m *MockEmitter) Close() error {
	return nil
}

func TestWorkerPool_ContinuesOnEmbeddingFailure(t *testing.T) {
	// 1. Setup
	embedder := &MockFailingEmbedder{}
	emitter := &MockEmitter{}
	workerPool := NewWorkerPool(1, embedder, emitter)
	
	workerPool.Start()
	
	// 2. Submit a file that we know has functions (so it triggers embedding)
	wd, _ := os.Getwd()
	// internal/ingest -> root is ../..
	fixturePath := filepath.Join(wd, "../../test/fixtures/typescript/sample.ts")
	
	workerPool.Submit(fixturePath)
	
	// 3. Stop and wait
	workerPool.Stop()
	
	// 4. Assert
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	
	if len(emitter.Nodes) == 0 {
		t.Fatalf("Expected nodes to be emitted even if embedding failed, but got 0. Note: This test is EXPECTED to fail before the fix.")
	}
}

func TestWorkerPool_EmitsFileAndDefinedInEdges(t *testing.T) {
	// 1. Setup
	embedder := &MockFailingEmbedder{} // Embedding doesn't matter here
	emitter := &MockEmitter{}
	workerPool := NewWorkerPool(1, embedder, emitter)
	
	workerPool.Start()
	
	// 2. Submit a file
	wd, _ := os.Getwd()
	fixturePath := filepath.Join(wd, "../../test/fixtures/typescript/sample.ts")
	workerPool.Submit(fixturePath)
	
	// 3. Stop and wait
	workerPool.Stop()
	
	// 4. Assert
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	
	foundFileNode := false
	var fileNodeID string
	
	for _, node := range emitter.Nodes {
		if node.Label == "File" {
			foundFileNode = true
			fileNodeID = node.ID
			
			if _, ok := node.Properties["file"]; !ok {
				t.Error("File node missing 'file' property")
			}
			if _, ok := node.Properties["name"]; !ok {
				t.Error("File node missing 'name' property")
			}
			break
		}
	}
	
	if !foundFileNode {
		t.Error("Expected to find a Node with Label 'File', but none found")
	}
	
	foundDefinedInEdge := false
	for _, edge := range emitter.Edges {
		if edge.Type == "DEFINED_IN" {
			foundDefinedInEdge = true
			if edge.TargetID != fileNodeID {
				t.Errorf("DEFINED_IN edge target %s does not match File node ID %s", edge.TargetID, fileNodeID)
			}
			break
		}
	}
	
	if !foundDefinedInEdge {
		t.Error("Expected to find an Edge with Type 'DEFINED_IN', but none found")
	}
}
