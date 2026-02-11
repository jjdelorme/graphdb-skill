package ingest

import (
	"fmt"
	"graphdb/internal/analysis"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/storage"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type WorkerPool struct {
	workers  int
	embedder embedding.Embedder
	emitter  storage.Emitter
	jobChan  chan string
	wg       sync.WaitGroup
}

func NewWorkerPool(workers int, embedder embedding.Embedder, emitter storage.Emitter) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		embedder: embedder,
		emitter:  emitter,
		jobChan:  make(chan string, 100),
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for path := range wp.jobChan {
		if err := wp.processFile(path); err != nil {
			log.Printf("Error processing file %s: %v", path, err)
		}
	}
}

func (wp *WorkerPool) processFile(path string) error {
	ext := filepath.Ext(path)
	parser, ok := analysis.GetParser(ext)
	if !ok {
		return nil // Not supported
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	nodes, edges, err := parser.Parse(path, content)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Create File Node
	fileNode := &graph.Node{
		ID:    path,
		Label: "File",
		Properties: map[string]interface{}{
			"file": path,
			"name": path,
		},
	}

	// Link nodes to File
	var definedInEdges []*graph.Edge
	for _, node := range nodes {
		if node.Label == "Function" || node.Label == "Method" || node.Label == "Class" {
			definedInEdges = append(definedInEdges, &graph.Edge{
				SourceID: node.ID,
				TargetID: fileNode.ID,
				Type:     "DEFINED_IN",
			})
		}
	}

	// Filter functions for embedding
	var functionNodes []*graph.Node
	var functionTexts []string

	for _, node := range nodes {
		if node.Label == "Function" || node.Label == "Method" {
			if name, ok := node.Properties["name"].(string); ok {
				functionNodes = append(functionNodes, node)
				functionTexts = append(functionTexts, name)
			}
		}
	}

	if len(functionTexts) > 0 {
		embeddings, err := wp.embedder.EmbedBatch(functionTexts)
		if err != nil {
			log.Printf("WARNING: failed to embed batch for %s: %v. Continuing without embeddings.", path, err)
		} else if len(embeddings) != len(functionNodes) {
			log.Printf("WARNING: embedding count mismatch for %s", path)
		} else {
			for i, node := range functionNodes {
				node.Properties["embedding"] = embeddings[i]
			}
		}
	}

	// Emit
	if err := wp.emitter.EmitNode(fileNode); err != nil {
		return fmt.Errorf("failed to emit file node: %w", err)
	}
	for _, edge := range definedInEdges {
		if err := wp.emitter.EmitEdge(edge); err != nil {
			return fmt.Errorf("failed to emit defined_in edge: %w", err)
		}
	}
	for _, node := range nodes {
		if err := wp.emitter.EmitNode(node); err != nil {
			return fmt.Errorf("failed to emit node: %w", err)
		}
	}
	for _, edge := range edges {
		if err := wp.emitter.EmitEdge(edge); err != nil {
			return fmt.Errorf("failed to emit edge: %w", err)
		}
	}

	return nil
}

func (wp *WorkerPool) Submit(filePath string) {
	wp.jobChan <- filePath
}

func (wp *WorkerPool) Stop() {
	close(wp.jobChan)
	wp.wg.Wait()
}
