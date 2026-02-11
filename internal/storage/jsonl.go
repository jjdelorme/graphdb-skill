package storage

import (
	"encoding/json"
	"graphdb/internal/graph"
	"io"
	"sync"
)

// JSONLEmitter implements the Emitter interface for writing JSONL files
// compatible with the legacy Neo4j loader.
type JSONLEmitter struct {
	w       io.Writer
	encoder *json.Encoder
	mu      sync.Mutex
}

// NewJSONLEmitter creates a new JSONLEmitter writing to w.
func NewJSONLEmitter(w io.Writer) *JSONLEmitter {
	return &JSONLEmitter{
		w:       w,
		encoder: json.NewEncoder(w),
	}
}

// SplitJSONLEmitter implements Emitter for writing nodes and edges to separate files.
type SplitJSONLEmitter struct {
	nodeEncoder *json.Encoder
	edgeEncoder *json.Encoder
	nodeCloser  io.Closer
	edgeCloser  io.Closer
}

// NewSplitJSONLEmitter creates a new SplitJSONLEmitter.
func NewSplitJSONLEmitter(nodeW, edgeW io.Writer) *SplitJSONLEmitter {
	s := &SplitJSONLEmitter{
		nodeEncoder: json.NewEncoder(nodeW),
		edgeEncoder: json.NewEncoder(edgeW),
	}
	if c, ok := nodeW.(io.Closer); ok {
		s.nodeCloser = c
	}
	if c, ok := edgeW.(io.Closer); ok {
		s.edgeCloser = c
	}
	return s
}

func (e *SplitJSONLEmitter) EmitNode(node *graph.Node) error {
	out := make(map[string]interface{})
	if node.Properties != nil {
		for k, v := range node.Properties {
			out[k] = v
		}
	}
	out["id"] = node.ID
	out["type"] = node.Label
	return e.nodeEncoder.Encode(out)
}

func (e *SplitJSONLEmitter) EmitEdge(edge *graph.Edge) error {
	out := map[string]string{
		"source": edge.SourceID,
		"target": edge.TargetID,
		"type":   edge.Type,
	}
	return e.edgeEncoder.Encode(out)
}

func (e *SplitJSONLEmitter) Close() error {
	if e.nodeCloser != nil {
		e.nodeCloser.Close()
	}
	if e.edgeCloser != nil {
		e.edgeCloser.Close()
	}
	return nil
}

// EmitNode writes a node to the output in the flattened JSON format required by import_to_neo4j.js.
// Map mappings:
// - node.ID -> "id"
// - node.Label -> "type"
// - node.Properties -> flattened into root object
func (e *JSONLEmitter) EmitNode(node *graph.Node) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make(map[string]interface{})

	// Copy properties first so they don't overwrite ID/Type if key collision exists (though they shouldn't)
	if node.Properties != nil {
		for k, v := range node.Properties {
			out[k] = v
		}
	}

	// Set core fields required by loader
	out["id"] = node.ID
	out["type"] = node.Label

	return e.encoder.Encode(out)
}

// EmitEdge writes an edge to the output.
// Map mappings:
// - edge.SourceID -> "source"
// - edge.TargetID -> "target"
// - edge.Type -> "type"
func (e *JSONLEmitter) EmitEdge(edge *graph.Edge) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := map[string]string{
		"source": edge.SourceID,
		"target": edge.TargetID,
		"type":   edge.Type,
	}
	return e.encoder.Encode(out)
}

// Close closes the underlying writer if it implements io.Closer.
func (e *JSONLEmitter) Close() error {
	if c, ok := e.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
