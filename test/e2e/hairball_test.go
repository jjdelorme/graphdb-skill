package e2e_test

import (
	"fmt"
	"math/rand"
	"testing"

	"graphdb/internal/analysis/leiden"
)

// generateHairballBenchmarkGraph creates a benchmark graph with distinct business clusters
// and dense, pervasive utility hub nodes connected to nearly all nodes in the graph.
func generateHairballBenchmarkGraph(
	numClusters int,
	nodesPerCluster int,
	numPervasiveHubs int,
	intraProb float64,
	interNoiseProb float64,
	seed int64,
) ([]string, []leiden.RawEdge, []string) {
	rng := rand.New(rand.NewSource(seed))
	totalDomainNodes := numClusters * nodesPerCluster

	var nodes []string
	domainNodeIDs := make([]string, totalDomainNodes)

	// 1. Create domain nodes
	for i := 0; i < totalDomainNodes; i++ {
		cID := i / nodesPerCluster
		nID := i % nodesPerCluster
		nodeName := fmt.Sprintf("domain_c%02d_func_%03d", cID, nID)
		nodes = append(nodes, nodeName)
		domainNodeIDs[i] = nodeName
	}

	// 2. Create pervasive utility hub nodes
	hubNodeIDs := make([]string, numPervasiveHubs)
	hubNames := []string{
		"infra_logger_log",
		"infra_app_config_get",
		"infra_db_connection_query",
		"infra_metrics_counter_inc",
		"infra_telemetry_tracer_span",
		"infra_security_auth_context",
		"infra_memory_cache_get",
		"infra_event_bus_publish",
	}
	for h := 0; h < numPervasiveHubs; h++ {
		name := fmt.Sprintf("hub_%d", h)
		if h < len(hubNames) {
			name = hubNames[h]
		}
		nodes = append(nodes, name)
		hubNodeIDs[h] = name
	}

	var edges []leiden.RawEdge

	// 3. Dense intra-cluster edges
	for c := 0; c < numClusters; c++ {
		start := c * nodesPerCluster
		end := start + nodesPerCluster
		for i := start; i < end; i++ {
			for j := i + 1; j < end; j++ {
				if rng.Float64() < intraProb {
					edges = append(edges, leiden.RawEdge{
						SourceID: domainNodeIDs[i],
						TargetID: domainNodeIDs[j],
						Type:     "CALLS",
					})
				}
			}
		}
	}

	// 4. Sparse inter-cluster background noise
	for i := 0; i < totalDomainNodes; i++ {
		cI := i / nodesPerCluster
		for j := i + 1; j < totalDomainNodes; j++ {
			cJ := j / nodesPerCluster
			if cI != cJ && rng.Float64() < interNoiseProb {
				edges = append(edges, leiden.RawEdge{
					SourceID: domainNodeIDs[i],
					TargetID: domainNodeIDs[j],
					Type:     "CALLS",
				})
			}
		}
	}

	// 5. Connect every pervasive hub to all domain nodes (degree = totalDomainNodes > 100)
	for _, hubID := range hubNodeIDs {
		for _, domainID := range domainNodeIDs {
			edges = append(edges, leiden.RawEdge{
				SourceID: domainID,
				TargetID: hubID,
				Type:     "CALLS",
			})
		}
	}

	return nodes, edges, hubNodeIDs
}

// TestE2E_HairballSuppression_500PlusNodes asserts that a benchmark graph containing 500+ nodes
// and dense high-degree utility hubs (degree > 100) partitions into >= 15 distinct communities
// with no single community exceeding 25% of total nodes when hub suppression is enabled.
func TestE2E_HairballSuppression_500PlusNodes(t *testing.T) {
	const (
		numClusters      = 20
		nodesPerCluster  = 30 // 600 domain nodes
		numPervasiveHubs = 5  // Total 605 nodes
		seed             = int64(42)
	)

	nodes, edges, hubIDs := generateHairballBenchmarkGraph(
		numClusters, nodesPerCluster, numPervasiveHubs, 0.45, 0.001, seed,
	)

	totalNodes := len(nodes)
	if totalNodes < 500 {
		t.Fatalf("Benchmark graph must have >= 500 nodes, got %d", totalNodes)
	}

	cfg := leiden.Config{
		Gamma:            0.0, // Auto-adaptive gamma search
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     true, // Hub suppression enabled (inverse-degree damping + quarantine)
		RandomSeed:       seed,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	engine := leiden.NewEngine(cfg, matrix)
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	t.Logf("Hairball Benchmark Result: %d communities, %d quarantined hubs, %d shared boundaries, gamma=%.4f",
		len(result.Communities), len(result.CrossCuttingHubs), len(result.SharedBoundaries), result.Gamma)

	// Assertion 1: Must produce >= 15 distinct communities
	if len(result.Communities) < 15 {
		t.Errorf("Expected >= 15 communities under hub suppression, got %d", len(result.Communities))
	}

	// Assertion 2: No single community may exceed 25% of total nodes
	maxAllowedSize := int(float64(totalNodes) * 0.25)
	for _, comm := range result.Communities {
		commSize := len(comm.NodeIDs)
		if commSize > maxAllowedSize {
			t.Errorf("Community %s exceeds 25%% size limit (%d nodes > %d max allowed)",
				comm.ID, commSize, maxAllowedSize)
		}
	}

	// Assertion 3: Pervasive high-degree hubs must be detected and quarantined
	quarantinedMap := make(map[string]*leiden.QuarantinedHubNode)
	for _, h := range result.CrossCuttingHubs {
		quarantinedMap[h.NodeID] = h
	}

	for _, expectedHubID := range hubIDs {
		hubInfo, found := quarantinedMap[expectedHubID]
		if !found {
			t.Errorf("Expected pervasive hub %s to be in CrossCuttingHubs quarantine", expectedHubID)
			continue
		}
		if hubInfo.Degree <= 100 {
			t.Errorf("Expected hub %s degree > 100, got %d", expectedHubID, hubInfo.Degree)
		}
		if hubInfo.HubScore <= 0.0 {
			t.Errorf("Expected positive hub z-score for %s, got %.2f", expectedHubID, hubInfo.HubScore)
		}
	}
}

// TestE2E_HairballSuppression_Extreme1000NodeGraph tests extreme scale with 1000+ nodes and 8 pervasive hubs.
func TestE2E_HairballSuppression_Extreme1000NodeGraph(t *testing.T) {
	const (
		numClusters      = 25
		nodesPerCluster  = 40 // 1000 domain nodes
		numPervasiveHubs = 8  // 1008 total nodes
		seed             = int64(2024)
	)

	nodes, edges, hubIDs := generateHairballBenchmarkGraph(
		numClusters, nodesPerCluster, numPervasiveHubs, 0.40, 0.0005, seed,
	)

	cfg := leiden.Config{
		Gamma:            0.035,
		MinCommunitySize: 25,
		MaxCommunitySize: 120,
		SuppressHubs:     true,
		RandomSeed:       seed,
		MaxIterations:    50,
		MaxHierDepth:     3,
	}
	matrix := leiden.DefaultEdgeWeightMatrix()

	engine := leiden.NewEngine(cfg, matrix)
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition failed: %v", err)
	}

	totalNodes := len(nodes)
	maxAllowedSize := int(float64(totalNodes) * 0.25)

	if len(result.Communities) < 15 {
		t.Errorf("Extreme graph: Expected >= 15 communities, got %d", len(result.Communities))
	}

	for _, comm := range result.Communities {
		if len(comm.NodeIDs) > maxAllowedSize {
			t.Errorf("Community %s size %d exceeds 25%% limit %d", comm.ID, len(comm.NodeIDs), maxAllowedSize)
		}
	}

	if len(result.CrossCuttingHubs) < len(hubIDs) {
		t.Errorf("Expected at least %d quarantined hubs, got %d", len(hubIDs), len(result.CrossCuttingHubs))
	}
}

// TestE2E_HairballSuppression_DampingEfficacy confirms that logarithmic edge damping
// prevents high-degree hubs from distorting local modularity.
func TestE2E_HairballSuppression_DampingEfficacy(t *testing.T) {
	nodes, edges, _ := generateHairballBenchmarkGraph(16, 35, 4, 0.45, 0.001, 789)

	// Run with suppression enabled
	cfgSuppressed := leiden.Config{
		Gamma:            0.03,
		MinCommunitySize: 20,
		MaxCommunitySize: 80,
		SuppressHubs:     true,
		RandomSeed:       42,
		MaxIterations:    40,
	}
	engineSuppressed := leiden.NewEngine(cfgSuppressed, leiden.DefaultEdgeWeightMatrix())
	resSuppressed, err := engineSuppressed.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Partition with suppression failed: %v", err)
	}

	// Verify >= 15 communities produced
	if len(resSuppressed.Communities) < 15 {
		t.Errorf("Suppressed partition produced only %d communities, expected >= 15", len(resSuppressed.Communities))
	}

	// Verify no cluster exceeds 25% of total nodes
	maxAllowed := int(float64(len(nodes)) * 0.25)
	for _, comm := range resSuppressed.Communities {
		if len(comm.NodeIDs) > maxAllowed {
			t.Errorf("Community %s exceeded 25%% limit: %d > %d", comm.ID, len(comm.NodeIDs), maxAllowed)
		}
	}
}
