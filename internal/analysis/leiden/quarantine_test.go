package leiden

import (
	"fmt"
	"testing"
)

func TestHubQuarantineIdentificationAndReintegration(t *testing.T) {
	// Construct a synthetic graph with 1 super-hub connected to 40 nodes,
	// and two cohesive communities of 20 nodes each (C1: n1..n20, C2: n21..n40).
	nodes := []string{"super_hub"}
	for i := 1; i <= 40; i++ {
		nodes = append(nodes, fmt.Sprintf("n%d", i))
	}

	edges := []RawEdge{}
	// Hub edges to all 40 nodes
	for i := 1; i <= 40; i++ {
		edges = append(edges, RawEdge{SourceID: "super_hub", TargetID: fmt.Sprintf("n%d", i), Type: "CALLS"})
	}

	// Internal edges within C1 (n1..n20) - dense clique-like connections
	for i := 1; i <= 20; i++ {
		for j := i + 1; j <= 20; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("n%d", i), TargetID: fmt.Sprintf("n%d", j), Type: "CALLS"})
		}
	}

	// Internal edges within C2 (n21..n40)
	for i := 21; i <= 40; i++ {
		for j := i + 1; j <= 40; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("n%d", i), TargetID: fmt.Sprintf("n%d", j), Type: "CALLS"})
		}
	}

	baseGraph := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), true)
	hq := IdentifyAndQuarantineHubs(baseGraph, true)

	if len(hq.Hubs) != 1 {
		t.Fatalf("Expected exactly 1 quarantined hub, got %d", len(hq.Hubs))
	}
	if hq.Hubs[0].NodeID != "super_hub" {
		t.Errorf("Expected super_hub to be quarantined, got %s", hq.Hubs[0].NodeID)
	}
	if hq.Hubs[0].Degree != 40 {
		t.Errorf("Expected hub degree 40, got %d", hq.Hubs[0].Degree)
	}

	// Active graph should not contain super_hub
	if _, exists := hq.ActiveGraph.IDToIndex["super_hub"]; exists {
		t.Errorf("Active graph should not contain super_hub")
	}

	// Run clustering on active graph
	activePart := LeidenClustering(hq.ActiveGraph, 0.1, 50, nil)
	uniqueComm := make(map[int]struct{})
	for _, c := range activePart {
		uniqueComm[c] = struct{}{}
	}

	if len(uniqueComm) < 2 {
		t.Errorf("Expected active graph to cleanly partition into at least 2 communities, got %d", len(uniqueComm))
	}

	// Build community objects
	comm1Nodes := []string{}
	comm2Nodes := []string{}
	for i := 1; i <= 20; i++ {
		comm1Nodes = append(comm1Nodes, fmt.Sprintf("n%d", i))
	}
	for i := 21; i <= 40; i++ {
		comm2Nodes = append(comm2Nodes, fmt.Sprintf("n%d", i))
	}

	communities := []*Community{
		{ID: "comm-0", NodeIDs: comm1Nodes, Size: int64(len(comm1Nodes))},
		{ID: "comm-1", NodeIDs: comm2Nodes, Size: int64(len(comm2Nodes))},
	}

	hq.ReintegrateHubs(baseGraph, communities)

	// Since super_hub connects equally to 20 nodes in comm-0 and 20 nodes in comm-1,
	// affinities should be approximately 0.50 to comm-0 and 0.50 to comm-1.
	aff0 := hq.Hubs[0].CommunityAffinities["comm-0"]
	aff1 := hq.Hubs[0].CommunityAffinities["comm-1"]

	if aff0 < 0.40 || aff0 > 0.60 {
		t.Errorf("Expected affinity to comm-0 ~0.50, got %f", aff0)
	}
	if aff1 < 0.40 || aff1 > 0.60 {
		t.Errorf("Expected affinity to comm-1 ~0.50, got %f", aff1)
	}
}

func TestHubDominantHostAssignment(t *testing.T) {
	// Construct a hub connected 90% to comm-0 and 10% to comm-1
	nodes := []string{"hub"}
	for i := 1; i <= 30; i++ {
		nodes = append(nodes, fmt.Sprintf("n%d", i))
	}

	edges := []RawEdge{}
	// Hub connected to 18 nodes in C1 (n1..n20) and 2 nodes in C2 (n21..n30)
	for i := 1; i <= 18; i++ {
		edges = append(edges, RawEdge{SourceID: "hub", TargetID: fmt.Sprintf("n%d", i), Type: "CALLS"})
	}
	for i := 21; i <= 22; i++ {
		edges = append(edges, RawEdge{SourceID: "hub", TargetID: fmt.Sprintf("n%d", i), Type: "CALLS"})
	}

	baseGraph := BuildGraph(nodes, edges, DefaultEdgeWeightMatrix(), false)
	hq := &HubQuarantine{
		Hubs: []*QuarantinedHubNode{
			{NodeID: "hub", Degree: 20, CommunityAffinities: make(map[string]float64)},
		},
		HubIndices:   map[int]struct{}{baseGraph.IDToIndex["hub"]: {}},
		ActiveGraph:  baseGraph,
		ActiveToOrig: nil,
		OrigToActive: nil,
	}

	comm1Nodes := []string{}
	for i := 1; i <= 20; i++ {
		comm1Nodes = append(comm1Nodes, fmt.Sprintf("n%d", i))
	}
	comm2Nodes := []string{}
	for i := 21; i <= 30; i++ {
		comm2Nodes = append(comm2Nodes, fmt.Sprintf("n%d", i))
	}

	communities := []*Community{
		{ID: "comm-0", NodeIDs: comm1Nodes, Size: int64(len(comm1Nodes))},
		{ID: "comm-1", NodeIDs: comm2Nodes, Size: int64(len(comm2Nodes))},
	}

	hq.ReintegrateHubs(baseGraph, communities)

	// Affinity to comm-0 should be 18 / 20 = 0.90 >= 0.70, triggering dominant host assignment
	aff0 := hq.Hubs[0].CommunityAffinities["comm-0"]
	if aff0 < 0.85 {
		t.Errorf("Expected affinity to comm-0 ~0.90, got %f", aff0)
	}

	foundInComm0 := false
	for _, id := range communities[0].NodeIDs {
		if id == "hub" {
			foundInComm0 = true
			break
		}
	}
	if !foundInComm0 {
		t.Errorf("Expected hub to be added to comm-0 due to >=0.70 affinity dominant host rule")
	}
}
