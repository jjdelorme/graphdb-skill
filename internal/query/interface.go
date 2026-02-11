package query

import "graphdb/internal/graph"

// Direction represents the direction of a relationship traversal.
type Direction int

const (
	Incoming Direction = iota
	Outgoing
	Both
)

// FeatureResult represents a result from a hybrid search (vector + structure).
type FeatureResult struct {
	Node  *graph.Node `json:"node"`
	Score float32     `json:"score"`
}

// NeighborResult represents the dependencies of a node (functions, globals).
type NeighborResult struct {
	Node         *graph.Node  `json:"node"`
	Dependencies []Dependency `json:"dependencies"`
}

// Dependency represents a dependency (function or global) with context.
type Dependency struct {
	Name string   `json:"name"`          // Name of the dependency (Function or Global)
	Type string   `json:"type"`          // "Function" or "Global"
	Via  []string `json:"via,omitempty"` // Trace path (for transitive globals)
}

// ImpactResult represents the upstream dependencies (callers).
type ImpactResult struct {
	Target  *graph.Node   `json:"target"`
	Callers []*graph.Node `json:"callers"`
	Paths   []*graph.Path `json:"paths"`
}

// GlobalUsageResult represents global variable usage.
type GlobalUsageResult struct {
	Target  *graph.Node   `json:"target"`
	Globals []*graph.Node `json:"globals"`
}

// SeamResult represents a suggested architectural seam (boundary).
type SeamResult struct {
	Seam string  `json:"seam"`
	File string  `json:"file"`
	Risk float64 `json:"risk"`
}

// GraphProvider defines the interface for graph database operations.
type GraphProvider interface {
	// Lifecycle
	Close() error

	// Core Operations
	FindNode(label string, property string, value string) (*graph.Node, error)
	Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error)

	// High-Level Features
	SearchFeatures(embedding []float32, limit int) ([]*FeatureResult, error)
	SearchSimilarFunctions(embedding []float32, limit int) ([]*FeatureResult, error)
	GetNeighbors(nodeID string, depth int) (*NeighborResult, error)
	GetCallers(nodeID string) ([]string, error)
	GetImpact(nodeID string, depth int) (*ImpactResult, error)
	GetGlobals(nodeID string) (*GlobalUsageResult, error)
	GetSeams(modulePattern string) ([]*SeamResult, error)
}
