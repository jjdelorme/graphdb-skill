package leiden

// RawEdge represents an input edge between two code entities with an edge type and optional base weight.
type RawEdge struct {
	SourceID string  `json:"source_id"`
	TargetID string  `json:"target_id"`
	Type     string  `json:"type"`
	Weight   float64 `json:"weight,omitempty"`
}

// EdgeWeightMatrix defines structural coupling base weights across relationship types.
type EdgeWeightMatrix struct {
	CallsWeight            float64 `json:"calls_weight"`             // default 1.0
	ContainsWeight         float64 `json:"contains_weight"`          // default 0.8
	InheritsWeight         float64 `json:"inherits_weight"`          // default 0.9
	UsesGlobalWeight       float64 `json:"uses_global_weight"`       // default 0.7
	CoChangedWeight        float64 `json:"co_changed_weight"`        // default 0.6
	ReferencesWeight       float64 `json:"references_weight"`        // default 0.5
	ImplicitSemanticWeight float64 `json:"implicit_semantic_weight"` // default 0.4
}

// DefaultEdgeWeightMatrix returns the standard base edge weight configuration.
func DefaultEdgeWeightMatrix() EdgeWeightMatrix {
	return EdgeWeightMatrix{
		CallsWeight:            1.0,
		ContainsWeight:         0.8,
		InheritsWeight:         0.9,
		UsesGlobalWeight:       0.7,
		CoChangedWeight:        0.6,
		ReferencesWeight:       0.5,
		ImplicitSemanticWeight: 0.4,
	}
}

// Config configures the CPM Leiden community detection engine.
type Config struct {
	Gamma            float64 `json:"gamma"`              // CPM resolution parameter (0.0 = adaptive search)
	MinCommunitySize int     `json:"min_community_size"` // default 30
	MaxCommunitySize int     `json:"max_community_size"` // default 250
	SuppressHubs     bool    `json:"suppress_hubs"`      // default true (IDF damping + top 1% quarantine)
	RandomSeed       int64   `json:"random_seed"`        // default 42
	MaxIterations    int     `json:"max_iterations"`     // default 50
	ResolutionSteps  int     `json:"resolution_steps"`   // default 8
	MaxHierDepth     int     `json:"max_hier_depth"`     // default 3
}

// DefaultConfig returns the standard CPM Leiden clustering configuration.
func DefaultConfig() Config {
	return Config{
		Gamma:            0.0, // Auto-adaptive search
		MinCommunitySize: 30,
		MaxCommunitySize: 250,
		SuppressHubs:     true,
		RandomSeed:       42,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}
}

// Community represents a detected structural community.
type Community struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Gamma             float64  `json:"gamma"`
	Size              int64    `json:"size"`
	Density           float64  `json:"density"`
	InternalEdgeCount int64    `json:"internal_edge_count"`
	BPRAvg            float64  `json:"bpr_avg"`
	NodeIDs           []string `json:"node_ids"`
	Level             int      `json:"level,omitempty"`
	ParentID          string   `json:"parent_id,omitempty"`
	InternalWeight    float64  `json:"internal_weight,omitempty"`
	TotalWeight       float64  `json:"total_weight,omitempty"`
}

// SharedBoundaryNode represents an interface or component bridging multiple communities (BPR >= 0.25).
type SharedBoundaryNode struct {
	NodeID                 string             `json:"node_id"`
	BPRMax                 float64            `json:"bpr_max"`
	BoundaryCommunityCount int                `json:"boundary_community_count"`
	CommunityBPRs          map[string]float64 `json:"community_bprs"`
}

// QuarantinedHubNode represents an isolated high-degree infrastructure hub (deg > mu + 3*sigma).
type QuarantinedHubNode struct {
	NodeID              string             `json:"node_id"`
	Degree              int                `json:"degree"`
	HubScore            float64            `json:"hub_score"`
	CommunityAffinities map[string]float64 `json:"community_affinities"`
}

// PartitionResult encapsulates the complete outcome of topological community detection.
type PartitionResult struct {
	Communities      []*Community          `json:"communities"`
	SharedBoundaries []*SharedBoundaryNode `json:"shared_boundaries"`
	CrossCuttingHubs []*QuarantinedHubNode `json:"cross_cutting_hubs"`
	Gamma            float64               `json:"gamma"`
	Quality          float64               `json:"quality"`
}

// Engine defines the primary contract for CPM Leiden community partitioning.
type Engine interface {
	Partition(nodes []string, edges []RawEdge) (*PartitionResult, error)
}

// WeightedNeighbor represents a directed/undirected adjacent neighbor and edge weight.
type WeightedNeighbor struct {
	Target int
	Weight float64
}

// Graph is the high-performance in-memory graph representation used across Leiden phases.
type Graph struct {
	NodeIDs     []string
	IDToIndex   map[string]int
	NodeWeights []float64            // w(v): node volume / size (1.0 for base graph)
	SelfLoops   []float64            // Internal self-loop weight accumulated during meta-aggregation
	Neighbors   [][]WeightedNeighbor // Adjacency list with consolidated weights
	Degrees     []int                // Structural unweighted degrees of base nodes
	TotalWeight float64              // Sum of all unique edge weights (undirected + self-loops)
}
