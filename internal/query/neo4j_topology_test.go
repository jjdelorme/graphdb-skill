package query

import (
	"graphdb/internal/graph"
	"testing"
)

func TestStructuralTopology_NodeAndEdgeGrouping(t *testing.T) {
	nodes := []*graph.Node{
		{
			ID:    "comm-1",
			Label: "StructuralCommunity",
			Properties: map[string]any{
				"name":                "Community 1",
				"gamma":               0.05,
				"size":                int64(45),
				"density":             0.25,
				"internal_edge_count": int64(100),
				"bpr_avg":             0.12,
			},
		},
		{
			ID:    "node-shared",
			Label: "SharedBoundary",
			Properties: map[string]any{
				"bpr_max":                  0.42,
				"boundary_community_count": 2,
			},
		},
		{
			ID:    "node-hub",
			Label: "CrossCuttingHub",
			Properties: map[string]any{
				"degree":    150,
				"hub_score": 3.8,
			},
		},
	}

	edges := []*graph.Edge{
		{
			SourceID: "fn1",
			TargetID: "comm-1",
			Type:     "IN_COMMUNITY",
		},
		{
			SourceID: "node-shared",
			TargetID: "comm-1",
			Type:     "BRIDGES",
			Properties: map[string]any{
				"ratio": 0.42,
			},
		},
		{
			SourceID: "node-hub",
			TargetID: "comm-1",
			Type:     "INFRASTRUCTURE_OF",
			Properties: map[string]any{
				"affinity": 0.9,
			},
		},
	}

	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
	if len(edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(edges))
	}
}
