package leiden

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestEnginePartitionEmptyGraph(t *testing.T) {
	engine := NewDefaultEngine()
	res, err := engine.Partition(nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error on empty graph: %v", err)
	}
	if len(res.Communities) != 0 {
		t.Errorf("Expected 0 communities on empty graph, got %d", len(res.Communities))
	}
}

func TestEnginePartitionSingleNode(t *testing.T) {
	engine := NewDefaultEngine()
	res, err := engine.Partition([]string{"nodeA"}, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(res.Communities) != 1 {
		t.Fatalf("Expected 1 community, got %d", len(res.Communities))
	}
	if res.Communities[0].Size != 1 || res.Communities[0].NodeIDs[0] != "nodeA" {
		t.Errorf("Unexpected community content: %+v", res.Communities[0])
	}
}

func TestEnginePartitionEndToEndMultiDomain(t *testing.T) {
	// Synthesize a realistic multi-domain codebase graph:
	// Domain A: Billing (30 nodes)
	// Domain B: Auth (30 nodes)
	// Domain C: Catalog (30 nodes)
	// Hub: AppLogger (connected to all nodes in A, B, C)
	// Shared Boundary: EventBus (connected to 10 nodes in A, 10 in B, 10 in C)
	nodes := []string{"AppLogger", "EventBus"}
	edges := []RawEdge{}

	// Add Domain A (Billing)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("billing_%d", i)
		nodes = append(nodes, id)
	}
	for i := 0; i < 30; i++ {
		edges = append(edges, RawEdge{
			SourceID: fmt.Sprintf("billing_%d", i),
			TargetID: fmt.Sprintf("billing_%d", (i+1)%30),
			Type:     "CALLS",
		})
		for j := i + 2; j < 30; j += 3 {
			edges = append(edges, RawEdge{
				SourceID: fmt.Sprintf("billing_%d", i),
				TargetID: fmt.Sprintf("billing_%d", j),
				Type:     "CALLS",
			})
		}
	}

	// Add Domain B (Auth)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("auth_%d", i)
		nodes = append(nodes, id)
	}
	for i := 0; i < 30; i++ {
		edges = append(edges, RawEdge{
			SourceID: fmt.Sprintf("auth_%d", i),
			TargetID: fmt.Sprintf("auth_%d", (i+1)%30),
			Type:     "CALLS",
		})
		for j := i + 2; j < 30; j += 3 {
			edges = append(edges, RawEdge{
				SourceID: fmt.Sprintf("auth_%d", i),
				TargetID: fmt.Sprintf("auth_%d", j),
				Type:     "CALLS",
			})
		}
	}

	// Add Domain C (Catalog)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("catalog_%d", i)
		nodes = append(nodes, id)
	}
	for i := 0; i < 30; i++ {
		edges = append(edges, RawEdge{
			SourceID: fmt.Sprintf("catalog_%d", i),
			TargetID: fmt.Sprintf("catalog_%d", (i+1)%30),
			Type:     "CALLS",
		})
		for j := i + 2; j < 30; j += 3 {
			edges = append(edges, RawEdge{
				SourceID: fmt.Sprintf("catalog_%d", i),
				TargetID: fmt.Sprintf("catalog_%d", j),
				Type:     "CALLS",
			})
		}
	}

	// Connect AppLogger (Hub) to all 90 nodes + EventBus
	for _, n := range nodes {
		if n != "AppLogger" {
			edges = append(edges, RawEdge{
				SourceID: "AppLogger",
				TargetID: n,
				Type:     "CALLS",
			})
		}
	}

	// Connect EventBus (Shared Boundary) to 8 nodes in each domain
	for i := 0; i < 8; i++ {
		edges = append(edges, RawEdge{SourceID: "EventBus", TargetID: fmt.Sprintf("billing_%d", i), Type: "CALLS"})
		edges = append(edges, RawEdge{SourceID: "EventBus", TargetID: fmt.Sprintf("auth_%d", i), Type: "CALLS"})
		edges = append(edges, RawEdge{SourceID: "EventBus", TargetID: fmt.Sprintf("catalog_%d", i), Type: "CALLS"})
	}

	cfg := DefaultConfig()
	cfg.RandomSeed = 42
	cfg.SuppressHubs = true
	cfg.MinCommunitySize = 20
	cfg.MaxCommunitySize = 250

	engine := NewEngine(cfg, DefaultEdgeWeightMatrix())
	res, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Engine Partition failed: %v", err)
	}

	// 1. Verify AppLogger was quarantined as CrossCuttingHub
	var foundHub bool
	for _, h := range res.CrossCuttingHubs {
		if h.NodeID == "AppLogger" {
			foundHub = true
			if len(h.CommunityAffinities) < 2 {
				t.Errorf("Expected AppLogger to link to multiple communities, got affinities: %v", h.CommunityAffinities)
			}
		}
	}
	if !foundHub {
		t.Errorf("Expected AppLogger to be quarantined as CrossCuttingHub")
	}

	// 2. Verify communities: Billing, Auth, Catalog should be cleanly separated
	if len(res.Communities) < 3 {
		t.Fatalf("Expected at least 3 distinct communities, got %d", len(res.Communities))
	}

	// Check that billing nodes are mostly in one community, auth in another, catalog in another
	commDomainCounts := make(map[string]map[string]int)
	for _, comm := range res.Communities {
		counts := make(map[string]int)
		for _, nodeID := range comm.NodeIDs {
			if len(nodeID) >= 7 && nodeID[:7] == "billing" {
				counts["billing"]++
			} else if len(nodeID) >= 4 && nodeID[:4] == "auth" {
				counts["auth"]++
			} else if len(nodeID) >= 7 && nodeID[:7] == "catalog" {
				counts["catalog"]++
			}
		}
		commDomainCounts[comm.ID] = counts
	}

	billingPure := false
	authPure := false
	catalogPure := false

	for _, counts := range commDomainCounts {
		if counts["billing"] >= 25 && counts["auth"] == 0 && counts["catalog"] == 0 {
			billingPure = true
		}
		if counts["auth"] >= 25 && counts["billing"] == 0 && counts["catalog"] == 0 {
			authPure = true
		}
		if counts["catalog"] >= 25 && counts["billing"] == 0 && counts["auth"] == 0 {
			catalogPure = true
		}
	}

	if !billingPure || !authPure || !catalogPure {
		t.Errorf("Community purity check failed! billingPure=%v, authPure=%v, catalogPure=%v, details=%v",
			billingPure, authPure, catalogPure, commDomainCounts)
	}

	// 3. Verify EventBus was detected as SharedBoundaryNode
	var foundBoundary bool
	for _, sb := range res.SharedBoundaries {
		if sb.NodeID == "EventBus" {
			foundBoundary = true
			if sb.BoundaryCommunityCount < 2 {
				t.Errorf("Expected EventBus to have BoundaryCommunityCount >= 2, got %d", sb.BoundaryCommunityCount)
			}
		}
	}
	if !foundBoundary {
		t.Errorf("Expected EventBus to be flagged as SharedBoundaryNode")
	}
}

func TestEngine10RunDeterminism(t *testing.T) {
	// 100-node graph with random-like connections
	nodes := []string{}
	for i := 0; i < 100; i++ {
		nodes = append(nodes, fmt.Sprintf("node_%d", i))
	}

	edges := []RawEdge{}
	for i := 0; i < 100; i++ {
		for j := i + 1; j < 100; j++ {
			if (i*13+j*17)%11 == 0 {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("node_%d", i),
					TargetID: fmt.Sprintf("node_%d", j),
					Type:     "CALLS",
				})
			}
		}
	}

	cfg := DefaultConfig()
	cfg.RandomSeed = 1337

	var baselineCommNodes [][]string

	for run := 0; run < 10; run++ {
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())
		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		currentCommNodes := make([][]string, len(res.Communities))
		for cIdx, comm := range res.Communities {
			sortedNodes := make([]string, len(comm.NodeIDs))
			copy(sortedNodes, comm.NodeIDs)
			sort.Strings(sortedNodes)
			currentCommNodes[cIdx] = sortedNodes
		}

		if run == 0 {
			baselineCommNodes = currentCommNodes
		} else {
			if !reflect.DeepEqual(baselineCommNodes, currentCommNodes) {
				t.Fatalf("Determinism failure on run %d! Partition differed from baseline.", run)
			}
		}
	}
}
