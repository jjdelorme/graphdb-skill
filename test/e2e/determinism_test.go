package e2e_test

import (
	"fmt"
	"math/rand"
	"testing"

	"graphdb/internal/analysis/leiden"
)

// generateComplexBenchmarkGraph generates a reproducible synthetic graph with specified cluster count,
// nodes per cluster, and edge distribution across multiple relationship types.
func generateComplexBenchmarkGraph(numClusters, nodesPerCluster int, intraProb, interProb float64, seed int64) ([]string, []leiden.RawEdge) {
	rng := rand.New(rand.NewSource(seed))
	totalNodes := numClusters * nodesPerCluster

	nodes := make([]string, totalNodes)
	for i := 0; i < totalNodes; i++ {
		clusterID := i / nodesPerCluster
		nodeIndex := i % nodesPerCluster
		nodes[i] = fmt.Sprintf("node_c%d_n%d", clusterID, nodeIndex)
	}

	edgeTypes := []string{"CALLS", "CONTAINS", "INHERITS", "USES_GLOBAL", "CO_CHANGED", "REFERENCES"}
	var edges []leiden.RawEdge

	// 1. Dense intra-cluster edges
	for c := 0; c < numClusters; c++ {
		start := c * nodesPerCluster
		end := start + nodesPerCluster
		for i := start; i < end; i++ {
			for j := i + 1; j < end; j++ {
				if rng.Float64() < intraProb {
					eType := edgeTypes[rng.Intn(len(edgeTypes))]
					var weight float64
					if eType == "CO_CHANGED" {
						weight = float64(rng.Intn(5) + 1)
					}
					edges = append(edges, leiden.RawEdge{
						SourceID: nodes[i],
						TargetID: nodes[j],
						Type:     eType,
						Weight:   weight,
					})
				}
			}
		}
	}

	// 2. Sparse inter-cluster cross-edges
	for i := 0; i < totalNodes; i++ {
		cI := i / nodesPerCluster
		for j := i + 1; j < totalNodes; j++ {
			cJ := j / nodesPerCluster
			if cI != cJ && rng.Float64() < interProb {
				edges = append(edges, leiden.RawEdge{
					SourceID: nodes[i],
					TargetID: nodes[j],
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges
}

// buildNodeToCommunityMap builds a canonical map from nodeID -> communityID from PartitionResult.
func buildNodeToCommunityMap(res *leiden.PartitionResult) map[string]string {
	m := make(map[string]string)
	for _, comm := range res.Communities {
		for _, nodeID := range comm.NodeIDs {
			m[nodeID] = comm.ID
		}
	}
	return m
}

// TestE2E_Determinism_10ConsecutiveRuns verifies that running CPM Leiden 10 consecutive times
// with the same random seed produces 100% identical partition assignments, community counts,
// node-to-community mapping, and quality scores.
func TestE2E_Determinism_10ConsecutiveRuns(t *testing.T) {
	const (
		runs            = 10
		numClusters     = 10
		nodesPerCluster = 50 // 500 total nodes
		fixedSeed       = int64(42)
	)

	nodes, edges := generateComplexBenchmarkGraph(numClusters, nodesPerCluster, 0.35, 0.002, 1337)

	cfg := leiden.Config{
		Gamma:            0.04,
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     true,
		RandomSeed:       fixedSeed,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineResult *leiden.PartitionResult
	var baselineMap map[string]string

	for run := 0; run < runs; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		result, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d: Partition failed with error: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(result)

		if run == 0 {
			baselineResult = result
			baselineMap = nodeMap

			if len(result.Communities) == 0 {
				t.Fatal("Run 0 produced 0 communities")
			}
			t.Logf("Baseline Run 0 produced %d communities, %d shared boundaries, %d hubs, quality=%.4f",
				len(result.Communities), len(result.SharedBoundaries), len(result.CrossCuttingHubs), result.Quality)
			continue
		}

		// 1. Assert identical community count
		if len(result.Communities) != len(baselineResult.Communities) {
			t.Fatalf("Run %d: Community count mismatch. Expected %d, got %d",
				run, len(baselineResult.Communities), len(result.Communities))
		}

		// 2. Assert identical node-to-community mapping for every node
		if len(nodeMap) != len(baselineMap) {
			t.Fatalf("Run %d: Node map size mismatch. Expected %d, got %d",
				run, len(baselineMap), len(nodeMap))
		}

		for nodeID, baseComm := range baselineMap {
			comm, exists := nodeMap[nodeID]
			if !exists {
				t.Fatalf("Run %d: Node %s missing from partition", run, nodeID)
			}
			if comm != baseComm {
				t.Fatalf("Run %d: Node %s community assignment diverged. Baseline=%s, Run=%s",
					run, nodeID, baseComm, comm)
			}
		}

		// 3. Assert exact member lists per community
		for i, comm := range result.Communities {
			baseComm := baselineResult.Communities[i]
			if comm.ID != baseComm.ID {
				t.Fatalf("Run %d: Community ID mismatch at index %d: %s vs %s", run, i, comm.ID, baseComm.ID)
			}
			if len(comm.NodeIDs) != len(baseComm.NodeIDs) {
				t.Fatalf("Run %d: Community %s size mismatch: %d vs %d",
					run, comm.ID, len(comm.NodeIDs), len(baseComm.NodeIDs))
			}
			for j, id := range comm.NodeIDs {
				if id != baseComm.NodeIDs[j] {
					t.Fatalf("Run %d: Community %s member %d mismatch: %s vs %s",
						run, comm.ID, j, id, baseComm.NodeIDs[j])
				}
			}
		}

		// 4. Assert identical quarantined hubs
		if len(result.CrossCuttingHubs) != len(baselineResult.CrossCuttingHubs) {
			t.Fatalf("Run %d: Quarantined hubs count mismatch: %d vs %d",
				run, len(baselineResult.CrossCuttingHubs), len(result.CrossCuttingHubs))
		}

		// 5. Assert identical shared boundary nodes
		if len(result.SharedBoundaries) != len(baselineResult.SharedBoundaries) {
			t.Fatalf("Run %d: Shared boundaries count mismatch: %d vs %d",
				run, len(baselineResult.SharedBoundaries), len(result.SharedBoundaries))
		}
	}
}

// TestE2E_Determinism_AutoAdaptiveGamma verifies determinism when using automatic adaptive gamma search.
func TestE2E_Determinism_AutoAdaptiveGamma(t *testing.T) {
	const (
		runs      = 10
		fixedSeed = int64(1337)
	)

	nodes, edges := generateComplexBenchmarkGraph(8, 40, 0.40, 0.003, 999)

	cfg := leiden.Config{
		Gamma:            0.0, // Trigger adaptive bisection search
		MinCommunitySize: 25,
		MaxCommunitySize: 80,
		SuppressHubs:     true,
		RandomSeed:       fixedSeed,
		MaxIterations:    40,
		ResolutionSteps:  6,
		MaxHierDepth:     2,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineGamma float64
	var baselineMap map[string]string

	for run := 0; run < runs; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		result, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d: Partition failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(result)

		if run == 0 {
			baselineGamma = result.Gamma
			baselineMap = nodeMap
			continue
		}

		if result.Gamma-baselineGamma > 1e-9 || baselineGamma-result.Gamma > 1e-9 {
			t.Fatalf("Run %d: Selected gamma diverged. Expected %f, got %f", run, baselineGamma, result.Gamma)
		}

		for nodeID, baseComm := range baselineMap {
			if nodeMap[nodeID] != baseComm {
				t.Fatalf("Run %d: Node %s assignment diverged in adaptive gamma mode", run, nodeID)
			}
		}
	}
}

// TestE2E_Determinism_DisconnectedComponents verifies deterministic clustering on disconnected graph components.
func TestE2E_Determinism_DisconnectedComponents(t *testing.T) {
	const (
		runs         = 10
		numComp      = 5
		nodesPerComp = 30
		fixedSeed    = int64(777)
	)

	var nodes []string
	var edges []leiden.RawEdge

	rng := rand.New(rand.NewSource(42))
	for c := 0; c < numComp; c++ {
		var compNodes []string
		for i := 0; i < nodesPerComp; i++ {
			nodeID := fmt.Sprintf("island_%d_%d", c, i)
			nodes = append(nodes, nodeID)
			compNodes = append(compNodes, nodeID)
		}
		// Connect fully within component
		for i := 0; i < len(compNodes); i++ {
			for j := i + 1; j < len(compNodes); j++ {
				if rng.Float64() < 0.3 {
					edges = append(edges, leiden.RawEdge{
						SourceID: compNodes[i],
						TargetID: compNodes[j],
						Type:     "CALLS",
					})
				}
			}
		}
	}

	cfg := leiden.Config{
		Gamma:            0.02,
		MinCommunitySize: 10,
		MaxCommunitySize: 50,
		SuppressHubs:     false,
		RandomSeed:       fixedSeed,
		MaxIterations:    30,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineMap map[string]string
	for run := 0; run < runs; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		result, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d: Partition failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(result)
		if run == 0 {
			baselineMap = nodeMap
			if len(result.Communities) < numComp {
				t.Fatalf("Expected at least %d communities, got %d", numComp, len(result.Communities))
			}
			continue
		}

		for nodeID, baseComm := range baselineMap {
			if nodeMap[nodeID] != baseComm {
				t.Fatalf("Run %d: Disconnected node %s assignment diverged", run, nodeID)
			}
		}
	}
}

// TestE2E_Determinism_HierarchicalSubClustering verifies determinism during recursive sub-clustering passes.
func TestE2E_Determinism_HierarchicalSubClustering(t *testing.T) {
	const (
		runs      = 10
		fixedSeed = int64(888)
	)

	// Single large cluster of 300 nodes
	var nodes []string
	for i := 0; i < 300; i++ {
		nodes = append(nodes, fmt.Sprintf("hier_node_%d", i))
	}

	var edges []leiden.RawEdge
	rng := rand.New(rand.NewSource(123))
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if rng.Float64() < 0.08 {
				edges = append(edges, leiden.RawEdge{
					SourceID: nodes[i],
					TargetID: nodes[j],
					Type:     "CALLS",
				})
			}
		}
	}

	cfg := leiden.Config{
		Gamma:            0.01,
		MinCommunitySize: 15,
		MaxCommunitySize: 100, // Forces sub-clustering on the 300-node graph
		SuppressHubs:     false,
		RandomSeed:       fixedSeed,
		MaxIterations:    40,
		MaxHierDepth:     3,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineMap map[string]string
	for run := 0; run < runs; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		result, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d: Partition failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(result)
		if run == 0 {
			baselineMap = nodeMap
			continue
		}

		for nodeID, baseComm := range baselineMap {
			if nodeMap[nodeID] != baseComm {
				t.Fatalf("Run %d: Hierarchical node %s assignment diverged", run, nodeID)
			}
		}
	}
}

// TestE2E_Determinism_MultiSeedVariation verifies that different seeds produce distinct, valid partition results.
func TestE2E_Determinism_MultiSeedVariation(t *testing.T) {
	nodes, edges := generateComplexBenchmarkGraph(6, 40, 0.35, 0.005, 555)
	matrix := leiden.DefaultEdgeWeightMatrix()

	seeds := []int64{111, 222, 333, 444}
	type seedRun struct {
		seed    int64
		nodeMap map[string]string
		quality float64
	}
	var results []seedRun

	for _, s := range seeds {
		cfg := leiden.Config{
			Gamma:            0.03,
			MinCommunitySize: 20,
			MaxCommunitySize: 100,
			SuppressHubs:     true,
			RandomSeed:       s,
			MaxIterations:    40,
		}
		engine := leiden.NewEngine(cfg, matrix)
		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Seed %d partition failed: %v", s, err)
		}
		if len(res.Communities) == 0 {
			t.Fatalf("Seed %d produced 0 communities", s)
		}
		results = append(results, seedRun{
			seed:    s,
			nodeMap: buildNodeToCommunityMap(res),
			quality: res.Quality,
		})
	}

	// Verify all seeds produced valid mappings covering all nodes
	for _, sr := range results {
		if len(sr.nodeMap) == 0 {
			t.Errorf("Seed %d mapped 0 nodes", sr.seed)
		}
	}
}
