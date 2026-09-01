package e2e_test

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"

	"graphdb/internal/analysis/leiden"
)

// ============================================================================
// 1. RESOLUTION LIMIT IMMUNITY STRESS TESTS (Ring of Cliques & Chain of Cliques)
// ============================================================================

// generateChainOfCliquesGraph builds an open linear chain of cliques:
// K_0 <-> K_1 <-> K_2 <-> ... <-> K_{numCliques-1}
func generateChainOfCliquesGraph(numCliques, cliqueSize, bridgeCount int) ([]string, []leiden.RawEdge, [][]string) {
	var nodes []string
	var edges []leiden.RawEdge
	cliques := make([][]string, numCliques)

	for c := 0; c < numCliques; c++ {
		var members []string
		for i := 0; i < cliqueSize; i++ {
			nodeID := fmt.Sprintf("chain_c%02d_n%02d", c, i)
			nodes = append(nodes, nodeID)
			members = append(members, nodeID)
		}
		cliques[c] = members

		// Complete intra-clique mesh
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

		// Connect to previous clique in chain
		if c > 0 {
			for b := 0; b < bridgeCount; b++ {
				src := cliques[c-1][b%cliqueSize]
				dst := members[b%cliqueSize]
				edges = append(edges, leiden.RawEdge{
					SourceID: src,
					TargetID: dst,
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
	}

	return nodes, edges, cliques
}

// TestStress_ResolutionLimit_LargeRingOfCliques50 tests 50 cliques of size 20 (1000 nodes) in a closed ring.
// Classic modularity (Newman-Girvan) merges adjacent cliques when graph size exceeds sqrt(2M).
// CPM Leiden MUST resolve exactly 50 distinct communities without merging.
func TestStress_ResolutionLimit_LargeRingOfCliques50(t *testing.T) {
	const (
		numCliques  = 50
		cliqueSize  = 20 // 1000 total nodes
		bridgeEdges = 2
	)

	nodes, edges, originalCliques := generateRingOfCliquesGraph(numCliques, cliqueSize, bridgeEdges)
	if len(nodes) != 1000 {
		t.Fatalf("Expected 1000 nodes, got %d", len(nodes))
	}

	cfg := leiden.Config{
		Gamma:            0.05,
		MinCommunitySize: 10,
		MaxCommunitySize: 50,
		SuppressHubs:     false,
		RandomSeed:       42,
		MaxIterations:    50,
	}
	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	if len(result.Communities) != numCliques {
		t.Fatalf("Resolution Limit Failure: Expected exactly %d communities for 50-ring, got %d",
			numCliques, len(result.Communities))
	}

	nodeToComm := buildNodeToCommunityMap(result)
	for cIdx, clique := range originalCliques {
		firstComm := nodeToComm[clique[0]]
		for _, member := range clique {
			if comm := nodeToComm[member]; comm != firstComm {
				t.Fatalf("Clique %d was split across communities: node %s in %s vs %s",
					cIdx, member, comm, firstComm)
			}
		}
	}
}

// TestStress_ResolutionLimit_LinearChainOfCliques50 tests a 50-clique open linear chain (1500 nodes).
func TestStress_ResolutionLimit_LinearChainOfCliques50(t *testing.T) {
	const (
		numCliques  = 50
		cliqueSize  = 30 // 1500 total nodes
		bridgeEdges = 1
	)

	nodes, edges, originalCliques := generateChainOfCliquesGraph(numCliques, cliqueSize, bridgeEdges)
	if len(nodes) != 1500 {
		t.Fatalf("Expected 1500 nodes, got %d", len(nodes))
	}

	cfg := leiden.Config{
		Gamma:            0.04,
		MinCommunitySize: 15,
		MaxCommunitySize: 60,
		SuppressHubs:     false,
		RandomSeed:       1337,
		MaxIterations:    50,
	}
	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	if len(result.Communities) != numCliques {
		t.Fatalf("Resolution Limit Failure: Expected %d communities on 50-chain, got %d",
			numCliques, len(result.Communities))
	}

	nodeToComm := buildNodeToCommunityMap(result)
	for cIdx, clique := range originalCliques {
		firstComm := nodeToComm[clique[0]]
		for _, member := range clique {
			if comm := nodeToComm[member]; comm != firstComm {
				t.Fatalf("Chain clique %d split: node %s in %s vs %s", cIdx, member, comm, firstComm)
			}
		}
	}
}

// TestStress_ResolutionLimit_QualityMonotonicity verifies that CPM quality strictly penalizes merging adjacent cliques.
func TestStress_ResolutionLimit_QualityMonotonicity(t *testing.T) {
	nodes, edges, _ := generateRingOfCliquesGraph(10, 20, 1)
	g := leiden.BuildGraph(nodes, edges, leiden.DefaultEdgeWeightMatrix(), false)

	// Ground truth partition: 10 distinct cliques
	optimalPartition := make([]int, len(nodes))
	for i := 0; i < len(nodes); i++ {
		optimalPartition[i] = i / 20
	}

	// Merged partition: merge clique 0 and clique 1 into a single community
	mergedPartition := make([]int, len(nodes))
	copy(mergedPartition, optimalPartition)
	for i := 0; i < 20; i++ {
		mergedPartition[i] = 1 // Merge clique 0 into clique 1
	}

	gamma := 0.05
	optQuality := leiden.CalculateQuality(g, optimalPartition, gamma)
	mergedQuality := leiden.CalculateQuality(g, mergedPartition, gamma)

	if optQuality <= mergedQuality {
		t.Fatalf("CPM Quality Error: Optimal partition quality (%.4f) should exceed merged partition quality (%.4f)",
			optQuality, mergedQuality)
	}
}

// ============================================================================
// 2. HAIRBALL SUPPRESSION STRESS TESTS (Multiple Extreme Hubs with Degree > 500)
// ============================================================================

// TestStress_HairballSuppression_MultipleExtremeHubs tests extreme hub topologies where
// multiple hubs connect to 500–1800 nodes across 40 business clusters (2000+ total nodes).
func TestStress_HairballSuppression_MultipleExtremeHubs(t *testing.T) {
	const (
		numClusters      = 40
		nodesPerCluster  = 50 // 2000 domain nodes
		numExtremeHubs   = 6  // 6 extreme hubs
		seed             = int64(9999)
	)

	rng := rand.New(rand.NewSource(seed))
	var nodes []string
	var domainIDs []string

	// Create 2000 domain nodes
	for c := 0; c < numClusters; c++ {
		for n := 0; n < nodesPerCluster; n++ {
			id := fmt.Sprintf("core_c%02d_func_%03d", c, n)
			nodes = append(nodes, id)
			domainIDs = append(domainIDs, id)
		}
	}

	// Create 6 extreme pervasive hubs
	hubIDs := []string{
		"global_extreme_logger_hub",
		"global_extreme_database_hub",
		"global_extreme_telemetry_hub",
		"global_extreme_security_hub",
		"global_extreme_config_hub",
		"global_extreme_event_bus_hub",
	}
	for _, hID := range hubIDs {
		nodes = append(nodes, hID)
	}

	var edges []leiden.RawEdge

	// 1. Dense intra-cluster edges
	for c := 0; c < numClusters; c++ {
		start := c * nodesPerCluster
		end := start + nodesPerCluster
		for i := start; i < end; i++ {
			for j := i + 1; j < end; j++ {
				if rng.Float64() < 0.35 {
					edges = append(edges, leiden.RawEdge{
						SourceID: domainIDs[i],
						TargetID: domainIDs[j],
						Type:     "CALLS",
					})
				}
			}
		}
	}

	// 2. Sparse inter-cluster cross noise
	for i := 0; i < len(domainIDs); i++ {
		cI := i / nodesPerCluster
		for j := i + 1; j < len(domainIDs); j++ {
			cJ := j / nodesPerCluster
			if cI != cJ && rng.Float64() < 0.0002 {
				edges = append(edges, leiden.RawEdge{
					SourceID: domainIDs[i],
					TargetID: domainIDs[j],
					Type:     "CALLS",
				})
			}
		}
	}

	// 3. Connect each extreme hub to 600..2000 domain nodes (degree > 500 guaranteed!)
	for hIdx, hID := range hubIDs {
		// Variable target fraction: 50% to 100% of all domain nodes
		targetFraction := 0.50 + float64(hIdx)*0.09
		for _, dID := range domainIDs {
			if rng.Float64() < targetFraction {
				edges = append(edges, leiden.RawEdge{
					SourceID: dID,
					TargetID: hID,
					Type:     "CALLS",
				})
			}
		}
	}

	totalNodes := len(nodes)
	if totalNodes != 2006 {
		t.Fatalf("Expected 2006 total nodes, got %d", totalNodes)
	}

	cfg := leiden.Config{
		Gamma:            0.0, // Adaptive gamma search
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     true, // Quarantine and damping enabled
		RandomSeed:       seed,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}

	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	t.Logf("Extreme Hairball Test: %d communities, %d hubs quarantined, %d shared boundaries, Gamma=%.5f",
		len(result.Communities), len(result.CrossCuttingHubs), len(result.SharedBoundaries), result.Gamma)

	// Check 1: Must partition into >= 20 communities (well over the >= 15 requirement)
	if len(result.Communities) < 15 {
		t.Fatalf("Hairball failure: Expected >= 15 communities, got %d", len(result.Communities))
	}

	// Check 2: No single community may exceed 25% of total nodes
	maxAllowedSize := int(float64(totalNodes) * 0.25)
	for _, comm := range result.Communities {
		if len(comm.NodeIDs) > maxAllowedSize {
			t.Fatalf("Hairball failure: Community %s size %d exceeds 25%% max (%d)",
				comm.ID, len(comm.NodeIDs), maxAllowedSize)
		}
	}

	// Check 3: Every extreme hub must be quarantined and have degree > 500
	quarantinedHubs := make(map[string]*leiden.QuarantinedHubNode)
	for _, hub := range result.CrossCuttingHubs {
		quarantinedHubs[hub.NodeID] = hub
	}

	for _, expectedHub := range hubIDs {
		hubInfo, found := quarantinedHubs[expectedHub]
		if !found {
			t.Fatalf("Expected extreme hub %s to be quarantined", expectedHub)
		}
		if hubInfo.Degree < 500 {
			t.Fatalf("Expected extreme hub %s degree > 500, got %d", expectedHub, hubInfo.Degree)
		}
		if hubInfo.HubScore <= 2.0 {
			t.Fatalf("Expected extreme hub %s z-score > 2.0, got %.2f", expectedHub, hubInfo.HubScore)
		}
	}
}

// ============================================================================
// 3. DETERMINISM STRESS TESTS (25+ Repeated Runs Across Various Modes)
// ============================================================================

// TestStress_Determinism_25RepeatedRuns runs 25 identical runs on a multi-relation graph.
func TestStress_Determinism_25RepeatedRuns(t *testing.T) {
	const (
		numRuns         = 25
		numClusters     = 12
		nodesPerCluster = 45 // 540 total nodes
		seed            = int64(7777)
	)

	nodes, edges := generateComplexBenchmarkGraph(numClusters, nodesPerCluster, 0.35, 0.002, 8888)

	cfg := leiden.Config{
		Gamma:            0.035,
		MinCommunitySize: 20,
		MaxCommunitySize: 90,
		SuppressHubs:     true,
		RandomSeed:       seed,
		MaxIterations:    40,
		MaxHierDepth:     3,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineResult *leiden.PartitionResult
	var baselineMap map[string]string

	for run := 0; run < numRuns; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(res)

		if run == 0 {
			baselineResult = res
			baselineMap = nodeMap
			if len(res.Communities) == 0 {
				t.Fatal("Run 0 produced 0 communities")
			}
			continue
		}

		// Exact equality checks
		if len(res.Communities) != len(baselineResult.Communities) {
			t.Fatalf("Run %d: Community count mismatch: %d vs %d", run, len(res.Communities), len(baselineResult.Communities))
		}
		if math.Abs(res.Quality-baselineResult.Quality) > 1e-9 {
			t.Fatalf("Run %d: Quality diverged: %.6f vs %.6f", run, res.Quality, baselineResult.Quality)
		}
		if math.Abs(res.Gamma-baselineResult.Gamma) > 1e-9 {
			t.Fatalf("Run %d: Gamma diverged: %.6f vs %.6f", run, res.Gamma, baselineResult.Gamma)
		}
		if len(res.CrossCuttingHubs) != len(baselineResult.CrossCuttingHubs) {
			t.Fatalf("Run %d: Hubs count mismatch: %d vs %d", run, len(res.CrossCuttingHubs), len(baselineResult.CrossCuttingHubs))
		}
		if len(res.SharedBoundaries) != len(baselineResult.SharedBoundaries) {
			t.Fatalf("Run %d: Boundary count mismatch: %d vs %d", run, len(res.SharedBoundaries), len(baselineResult.SharedBoundaries))
		}

		for nodeID, baseComm := range baselineMap {
			if nodeMap[nodeID] != baseComm {
				t.Fatalf("Run %d: Node %s community diverged (%s vs %s)", run, nodeID, nodeMap[nodeID], baseComm)
			}
		}
	}
}

// TestStress_Determinism_25AdaptiveGammaRuns runs 25 consecutive runs with adaptive gamma search.
func TestStress_Determinism_25AdaptiveGammaRuns(t *testing.T) {
	const (
		numRuns = 25
		seed    = int64(12345)
	)

	nodes, edges := generateComplexBenchmarkGraph(10, 35, 0.40, 0.003, 54321)

	cfg := leiden.Config{
		Gamma:            0.0, // Adaptive search
		MinCommunitySize: 20,
		MaxCommunitySize: 75,
		SuppressHubs:     true,
		RandomSeed:       seed,
		MaxIterations:    40,
		ResolutionSteps:  6,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	var baselineGamma float64
	var baselineMap map[string]string

	for run := 0; run < numRuns; run++ {
		engine := leiden.NewEngine(cfg, matrix)
		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Adaptive run %d failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(res)
		if run == 0 {
			baselineGamma = res.Gamma
			baselineMap = nodeMap
			continue
		}

		if math.Abs(res.Gamma-baselineGamma) > 1e-9 {
			t.Fatalf("Adaptive run %d: Gamma diverged (%.6f vs %.6f)", run, res.Gamma, baselineGamma)
		}

		for nodeID, baseComm := range baselineMap {
			if nodeMap[nodeID] != baseComm {
				t.Fatalf("Adaptive run %d: Node %s assignment diverged", run, nodeID)
			}
		}
	}
}

// ============================================================================
// 4. EDGE CASE & ADVERSARIAL TOPOLOGIES
// ============================================================================

// TestStress_EdgeCases_DisconnectedStars tests multiple disconnected star graphs.
func TestStress_EdgeCases_DisconnectedStars(t *testing.T) {
	const numStars = 8
	const leavesPerStar = 25

	var nodes []string
	var edges []leiden.RawEdge

	for s := 0; s < numStars; s++ {
		center := fmt.Sprintf("star_%d_center", s)
		nodes = append(nodes, center)
		for l := 0; l < leavesPerStar; l++ {
			leaf := fmt.Sprintf("star_%d_leaf_%d", s, l)
			nodes = append(nodes, leaf)
			edges = append(edges, leiden.RawEdge{
				SourceID: center,
				TargetID: leaf,
				Type:     "CALLS",
			})
		}
	}

	cfg := leiden.Config{
		Gamma:            0.01,
		MinCommunitySize: 10,
		MaxCommunitySize: 50,
		SuppressHubs:     false,
		RandomSeed:       42,
		MaxIterations:    30,
	}
	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Disconnected stars failed: %v", err)
	}

	if len(result.Communities) < numStars {
		t.Fatalf("Expected at least %d communities for disconnected stars, got %d",
			numStars, len(result.Communities))
	}
}

// TestStress_EdgeCases_CompleteBipartite tests clustering behavior on K_{50,50}.
func TestStress_EdgeCases_CompleteBipartite(t *testing.T) {
	var nodes []string
	var edges []leiden.RawEdge

	for i := 0; i < 50; i++ {
		nodes = append(nodes, fmt.Sprintf("left_%d", i))
		nodes = append(nodes, fmt.Sprintf("right_%d", i))
	}

	for i := 0; i < 50; i++ {
		for j := 0; j < 50; j++ {
			edges = append(edges, leiden.RawEdge{
				SourceID: fmt.Sprintf("left_%d", i),
				TargetID: fmt.Sprintf("right_%d", j),
				Type:     "CALLS",
			})
		}
	}

	cfg := leiden.Config{
		Gamma:            0.02,
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     false,
		RandomSeed:       42,
		MaxIterations:    30,
	}
	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Complete bipartite failed: %v", err)
	}
	if len(result.Communities) == 0 {
		t.Fatal("Expected at least 1 community for complete bipartite")
	}
}

// TestStress_EdgeCases_AllSingletons tests 100 nodes with 0 edges.
func TestStress_EdgeCases_AllSingletons(t *testing.T) {
	var nodes []string
	for i := 0; i < 100; i++ {
		nodes = append(nodes, fmt.Sprintf("singleton_%d", i))
	}

	engine := leiden.NewDefaultEngine()
	result, err := engine.Partition(nodes, nil)
	if err != nil {
		t.Fatalf("All singletons partition failed: %v", err)
	}

	if len(result.Communities) != 100 {
		t.Fatalf("Expected 100 singletons, got %d", len(result.Communities))
	}
	for _, comm := range result.Communities {
		if comm.Size != 1 {
			t.Errorf("Expected community size 1, got %d", comm.Size)
		}
	}
}

// TestStress_EdgeCases_SelfLoopsOnly tests nodes with only self-loops.
func TestStress_EdgeCases_SelfLoopsOnly(t *testing.T) {
	nodes := []string{"self1", "self2", "self3"}
	edges := []leiden.RawEdge{
		{SourceID: "self1", TargetID: "self1", Type: "CALLS", Weight: 2.0},
		{SourceID: "self2", TargetID: "self2", Type: "CALLS", Weight: 3.0},
		{SourceID: "self3", TargetID: "self3", Type: "CALLS", Weight: 1.0},
	}

	engine := leiden.NewDefaultEngine()
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Self loops only failed: %v", err)
	}
	if len(result.Communities) != 3 {
		t.Fatalf("Expected 3 communities, got %d", len(result.Communities))
	}
}

// TestStress_Concurrency_ParallelPartitions tests concurrent execution of Leiden partition across 20 goroutines.
func TestStress_Concurrency_ParallelPartitions(t *testing.T) {
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			nodes, edges := generateComplexBenchmarkGraph(6, 30, 0.3, 0.002, int64(100+gid))
			cfg := leiden.Config{
				Gamma:            0.04,
				MinCommunitySize: 15,
				MaxCommunitySize: 60,
				SuppressHubs:     true,
				RandomSeed:       int64(100 + gid),
				MaxIterations:    30,
			}
			engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
			res, err := engine.Partition(nodes, edges)
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", gid, err)
				return
			}
			if len(res.Communities) == 0 {
				t.Errorf("Goroutine %d produced 0 communities", gid)
			}
		}(g)
	}

	wg.Wait()
}
