package leiden

import (
	"fmt"
	"math"
	"testing"
)

func TestAnalyzeBPRSharedBoundary(t *testing.T) {
	// Create two 20-node communities and 1 bridge node "shared_service"
	// "shared_service" connects to 5 nodes in Comm 0 and 5 nodes in Comm 1 equally with weight 1.0.
	nodes := []string{"shared_service"}
	for i := 0; i < 40; i++ {
		nodes = append(nodes, fmt.Sprintf("node_%d", i))
	}

	edges := []RawEdge{}
	// Comm 0 internal edges (nodes 0..19)
	for i := 0; i < 20; i++ {
		for j := i + 1; j < 20; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("node_%d", i), TargetID: fmt.Sprintf("node_%d", j), Type: "CALLS"})
		}
	}
	// Comm 1 internal edges (nodes 20..39)
	for i := 20; i < 40; i++ {
		for j := i + 1; j < 40; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("node_%d", i), TargetID: fmt.Sprintf("node_%d", j), Type: "CALLS"})
		}
	}

	// Connect shared_service to 5 nodes in Comm 0 and 5 nodes in Comm 1 with equal weight
	for i := 0; i < 5; i++ {
		edges = append(edges, RawEdge{SourceID: "shared_service", TargetID: fmt.Sprintf("node_%d", i), Type: "CALLS"})
		edges = append(edges, RawEdge{SourceID: "shared_service", TargetID: fmt.Sprintf("node_%d", i+20), Type: "CALLS"})
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	comm0Nodes := []string{}
	for i := 0; i < 20; i++ {
		comm0Nodes = append(comm0Nodes, fmt.Sprintf("node_%d", i))
	}
	comm1Nodes := []string{}
	for i := 20; i < 40; i++ {
		comm1Nodes = append(comm1Nodes, fmt.Sprintf("node_%d", i))
	}

	// Put shared_service in Comm 0 for the test
	comm0Nodes = append(comm0Nodes, "shared_service")

	communities := []*Community{
		{ID: "comm-0", NodeIDs: comm0Nodes, Size: int64(len(comm0Nodes))},
		{ID: "comm-1", NodeIDs: comm1Nodes, Size: int64(len(comm1Nodes))},
	}

	sharedBoundaries, avgBPRs := AnalyzeBPR(g, communities)

	if len(sharedBoundaries) != 1 {
		t.Fatalf("Expected exactly 1 shared boundary node, got %d", len(sharedBoundaries))
	}

	sb := sharedBoundaries[0]
	if sb.NodeID != "shared_service" {
		t.Errorf("Expected shared_service, got %s", sb.NodeID)
	}
	if sb.BoundaryCommunityCount != 2 {
		t.Errorf("Expected BoundaryCommunityCount 2, got %d", sb.BoundaryCommunityCount)
	}

	bpr0 := sb.CommunityBPRs["comm-0"]
	bpr1 := sb.CommunityBPRs["comm-1"]

	if math.Abs(bpr0-0.5) > 1e-6 {
		t.Errorf("Expected BPR(comm-0) = 0.5, got %f", bpr0)
	}
	if math.Abs(bpr1-0.5) > 1e-6 {
		t.Errorf("Expected BPR(comm-1) = 0.5, got %f", bpr1)
	}

	if avgBPRs["comm-0"] <= 0 || avgBPRs["comm-1"] <= 0 {
		t.Errorf("Expected positive average BPRs, got %v", avgBPRs)
	}
}

func TestAnalyzeBPRPureInternalNode(t *testing.T) {
	// A node strictly inside Comm 0 with no external connections should have BPR(comm-0) = 1.0,
	// and should NOT be flagged as a shared boundary node.
	nodes := []string{"n0", "n1", "n2"}
	edges := []RawEdge{
		{SourceID: "n0", TargetID: "n1", Type: "CALLS"},
		{SourceID: "n1", TargetID: "n2", Type: "CALLS"},
	}

	g := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)

	communities := []*Community{
		{ID: "comm-0", NodeIDs: []string{"n0", "n1", "n2"}, Size: 3},
		{ID: "comm-1", NodeIDs: []string{}, Size: 0},
	}

	sharedBoundaries, _ := AnalyzeBPR(g, communities)
	if len(sharedBoundaries) != 0 {
		t.Errorf("Expected 0 shared boundary nodes, got %d", len(sharedBoundaries))
	}
}
