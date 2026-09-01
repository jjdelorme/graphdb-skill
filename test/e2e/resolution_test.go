package e2e_test

import (
	"fmt"
	"testing"

	"graphdb/internal/analysis/leiden"
)

// generateRingOfCliquesGraph generates a classical "Ring of Cliques" benchmark graph.
// It constructs numCliques distinct cliques of cliqueSize nodes each, connected in a closed ring
// by bridgeCount bridge edges between adjacent cliques K_i <-> K_{(i+1)%numCliques}.
func generateRingOfCliquesGraph(numCliques, cliqueSize, bridgeCount int) ([]string, []leiden.RawEdge, [][]string) {
	var nodes []string
	var edges []leiden.RawEdge
	cliqueNodes := make([][]string, numCliques)

	// 1. Generate nodes and intra-clique full mesh edges
	for c := 0; c < numCliques; c++ {
		var members []string
		for i := 0; i < cliqueSize; i++ {
			nodeID := fmt.Sprintf("submod_%02d_func_%02d", c, i)
			nodes = append(nodes, nodeID)
			members = append(members, nodeID)
		}
		cliqueNodes[c] = members

		// Create complete intra-clique graph (K_cliqueSize)
		for i := 0; i < cliqueSize; i++ {
			for j := i + 1; j < cliqueSize; j++ {
				edges = append(edges, leiden.RawEdge{
					SourceID: members[i],
					TargetID: members[j],
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
	}

	// 2. Connect adjacent cliques in a ring with sparse bridge edges
	for c := 0; c < numCliques; c++ {
		nextC := (c + 1) % numCliques
		for b := 0; b < bridgeCount; b++ {
			srcNode := cliqueNodes[c][b%cliqueSize]
			dstNode := cliqueNodes[nextC][b%cliqueSize]
			edges = append(edges, leiden.RawEdge{
				SourceID: srcNode,
				TargetID: dstNode,
				Type:     "CALLS",
				Weight:   1.0,
			})
		}
	}

	return nodes, edges, cliqueNodes
}

// TestE2E_ResolutionLimitImmunity_30Submodules asserts that CPM Leiden preserves
// all 30 independent synthetic submodules (cliques of 30 nodes connected by sparse cross-links)
// as 30 distinct communities without collapsing or merging them (Resolution Limit Immunity).
func TestE2E_ResolutionLimitImmunity_30Submodules(t *testing.T) {
	const (
		numSubmodules = 30
		cliqueSize    = 30 // Total 900 nodes
		bridgeEdges   = 2
	)

	nodes, edges, originalCliques := generateRingOfCliquesGraph(numSubmodules, cliqueSize, bridgeEdges)

	totalNodes := len(nodes)
	if totalNodes != numSubmodules*cliqueSize {
		t.Fatalf("Expected %d total nodes, got %d", numSubmodules*cliqueSize, totalNodes)
	}

	cfg := leiden.Config{
		Gamma:            0.04, // CPM resolution parameter
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     false,
		RandomSeed:       42,
		MaxIterations:    50,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	engine := leiden.NewEngine(cfg, matrix)
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	t.Logf("Ring of Cliques Result: %d communities, %d shared boundaries, quality=%.4f",
		len(result.Communities), len(result.SharedBoundaries), result.Quality)

	// Assertion 1: Must recover exactly 30 distinct communities
	if len(result.Communities) != numSubmodules {
		t.Fatalf("Resolution Limit Failure: CPM Leiden produced %d communities, expected exactly %d",
			len(result.Communities), numSubmodules)
	}

	// Assertion 2: Each community must have exactly cliqueSize (30) nodes
	for _, comm := range result.Communities {
		if len(comm.NodeIDs) != cliqueSize {
			t.Errorf("Community %s has %d nodes, expected exactly %d", comm.ID, len(comm.NodeIDs), cliqueSize)
		}
	}

	// Assertion 3: Each original clique must be 100% intact within a single community
	nodeToComm := buildNodeToCommunityMap(result)
	for cIdx, clique := range originalCliques {
		firstNodeComm := nodeToComm[clique[0]]
		for _, nodeID := range clique {
			assignedComm := nodeToComm[nodeID]
			if assignedComm != firstNodeComm {
				t.Errorf("Submodule %d fragmented: node %s in comm %s, expected %s",
					cIdx, nodeID, assignedComm, firstNodeComm)
			}
		}
	}
}

// TestE2E_ResolutionLimitImmunity_VariableSizes asserts resolution limit immunity
// with heterogeneous submodule sizes ranging between 25 and 40 nodes.
func TestE2E_ResolutionLimitImmunity_VariableSizes(t *testing.T) {
	const numSubmodules = 30
	submoduleSizes := make([]int, numSubmodules)
	for i := 0; i < numSubmodules; i++ {
		// Cycle sizes: 25, 28, 30, 35, 40
		switch i % 5 {
		case 0:
			submoduleSizes[i] = 25
		case 1:
			submoduleSizes[i] = 28
		case 2:
			submoduleSizes[i] = 30
		case 3:
			submoduleSizes[i] = 35
		case 4:
			submoduleSizes[i] = 40
		}
	}

	var nodes []string
	var edges []leiden.RawEdge
	originalModules := make([][]string, numSubmodules)

	for c := 0; c < numSubmodules; c++ {
		size := submoduleSizes[c]
		var members []string
		for i := 0; i < size; i++ {
			nodeID := fmt.Sprintf("var_submod_%02d_func_%02d", c, i)
			nodes = append(nodes, nodeID)
			members = append(members, nodeID)
		}
		originalModules[c] = members

		// Dense intra-module calls (dense clique)
		for i := 0; i < size; i++ {
			for j := i + 1; j < size; j++ {
				edges = append(edges, leiden.RawEdge{
					SourceID: members[i],
					TargetID: members[j],
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
	}

	// Connect adjacent modules with 1 sparse cross-link
	for c := 0; c < numSubmodules; c++ {
		nextC := (c + 1) % numSubmodules
		edges = append(edges, leiden.RawEdge{
			SourceID: originalModules[c][0],
			TargetID: originalModules[nextC][0],
			Type:     "CALLS",
			Weight:   1.0,
		})
	}

	cfg := leiden.Config{
		Gamma:            0.035,
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     false,
		RandomSeed:       1337,
		MaxIterations:    50,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	engine := leiden.NewEngine(cfg, matrix)
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	if len(result.Communities) != numSubmodules {
		t.Fatalf("Expected %d communities for variable-sized submodules, got %d",
			numSubmodules, len(result.Communities))
	}

	nodeToComm := buildNodeToCommunityMap(result)
	for cIdx, members := range originalModules {
		expectedSize := submoduleSizes[cIdx]
		firstComm := nodeToComm[members[0]]

		for _, m := range members {
			if nodeToComm[m] != firstComm {
				t.Errorf("Variable submodule %d (size %d) split across communities", cIdx, expectedSize)
			}
		}
	}
}

// TestE2E_ResolutionLimitImmunity_BPRSharedBoundaries asserts that bridge nodes
// between adjacent submodules have correctly calculated Boundary Participation Ratios.
func TestE2E_ResolutionLimitImmunity_BPRSharedBoundaries(t *testing.T) {
	nodes, edges, _ := generateRingOfCliquesGraph(10, 30, 2)

	cfg := leiden.Config{
		Gamma:            0.04,
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     false,
		RandomSeed:       42,
		MaxIterations:    40,
	}
	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	if len(result.Communities) != 10 {
		t.Fatalf("Expected 10 communities, got %d", len(result.Communities))
	}

	// Verify all communities have valid density and size
	for _, comm := range result.Communities {
		if comm.Size != 30 {
			t.Errorf("Community %s size = %d, expected 30", comm.ID, comm.Size)
		}
		if comm.Density <= 0.95 {
			t.Errorf("Community %s clique density = %.4f, expected near 1.0", comm.ID, comm.Density)
		}
	}
}
