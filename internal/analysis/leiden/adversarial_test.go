package leiden

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

// Helper: verify that all input nodes are partitioned without loss or duplication.
func verifyPartitionIntegrity(t *testing.T, nodes []string, res *PartitionResult) {
	t.Helper()
	nodeSet := make(map[string]int)
	for _, n := range nodes {
		nodeSet[n] = 0
	}

	totalInComms := 0
	for _, comm := range res.Communities {
		for _, nID := range comm.NodeIDs {
			if _, exists := nodeSet[nID]; !exists {
				t.Fatalf("Node %s in community %s was not in input node list", nID, comm.ID)
			}
			nodeSet[nID]++
			totalInComms++
		}
	}

	for nID, count := range nodeSet {
		if count == 0 {
			t.Fatalf("Node %s was dropped from all communities", nID)
		}
		if count > 1 {
			t.Fatalf("Node %s appears %d times across communities", nID, count)
		}
	}

	if totalInComms != len(nodes) {
		t.Fatalf("Expected %d nodes across communities, got %d", len(nodes), totalInComms)
	}
}

// 1. Gamma Parameter Extremes (gamma -> 0, gamma -> infinity, negative, zero)
func TestAdversarial_GammaExtremes(t *testing.T) {
	// Build a connected graph of 3 distinct cliques of size 10 connected by single bridge edges
	nodes := make([]string, 30)
	for i := 0; i < 30; i++ {
		nodes[i] = fmt.Sprintf("n_%d", i)
	}

	var edges []RawEdge
	// Clique 1: 0..9, Clique 2: 10..19, Clique 3: 20..29
	for c := 0; c < 3; c++ {
		start := c * 10
		for i := start; i < start+10; i++ {
			for j := i + 1; j < start+10; j++ {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("n_%d", i),
					TargetID: fmt.Sprintf("n_%d", j),
					Type:     "CALLS",
				})
			}
		}
	}
	// Bridges
	edges = append(edges, RawEdge{SourceID: "n_9", TargetID: "n_10", Type: "CALLS"})
	edges = append(edges, RawEdge{SourceID: "n_19", TargetID: "n_20", Type: "CALLS"})

	// Scenario A: Ultra-low gamma (1e-9) -> should merge all connected nodes into 1 community
	t.Run("UltraLowGamma_MergesAll", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = 1e-9
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on ultra-low gamma: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if len(res.Communities) != 1 {
			t.Errorf("Expected 1 merged community for gamma=1e-9 on connected graph, got %d", len(res.Communities))
		}
	})

	// Scenario B: Ultra-high gamma (1e6) -> should split all nodes into singletons (30 communities)
	t.Run("UltraHighGamma_AllSingletons", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = 1e6
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on ultra-high gamma: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if len(res.Communities) != 30 {
			t.Errorf("Expected 30 singleton communities for gamma=1e6, got %d", len(res.Communities))
		}
		for _, c := range res.Communities {
			if c.Size != 1 {
				t.Errorf("Expected community size 1, got %d", c.Size)
			}
		}
	})

	// Scenario C: Moderate resolution (gamma = 0.1) -> should cleanly detect 3 cliques
	t.Run("ModerateGamma_DetectsCliques", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = 0.1
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on moderate gamma: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if len(res.Communities) != 3 {
			t.Errorf("Expected 3 communities for gamma=0.1, got %d", len(res.Communities))
		}
	})

	// Scenario D: Gamma = 0.0 (triggers adaptive search)
	t.Run("AdaptiveGamma_ZeroExplicit", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = 0.0
		cfg.MinCommunitySize = 5
		cfg.MaxCommunitySize = 15
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on adaptive gamma: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if res.Gamma <= 0 {
			t.Errorf("Expected positive resolved gamma from adaptive search, got %f", res.Gamma)
		}
	})

	// Scenario E: Negative Gamma (should fallback safely to adaptive search)
	t.Run("NegativeGamma_Fallback", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = -5.0
		cfg.MinCommunitySize = 5
		cfg.MaxCommunitySize = 15
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on negative gamma: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if res.Gamma <= 0 {
			t.Errorf("Expected positive resolved gamma, got %f", res.Gamma)
		}
	})
}

// 2. Edge Weight Matrix Scaling & Resolution
func TestAdversarial_EdgeWeightMatrixScaling(t *testing.T) {
	nodes := []string{"A", "B", "C", "D", "E", "F"}

	// Scenario A: Ultra-large weights (1e12)
	t.Run("UltraLargeWeights", func(t *testing.T) {
		edges := []RawEdge{
			{SourceID: "A", TargetID: "B", Type: "CALLS", Weight: 1e12},
			{SourceID: "B", TargetID: "C", Type: "CALLS", Weight: 1e12},
			{SourceID: "C", TargetID: "A", Type: "CALLS", Weight: 1e12},
			{SourceID: "D", TargetID: "E", Type: "CALLS", Weight: 1e12},
			{SourceID: "E", TargetID: "F", Type: "CALLS", Weight: 1e12},
			{SourceID: "F", TargetID: "D", Type: "CALLS", Weight: 1e12},
			{SourceID: "C", TargetID: "D", Type: "CALLS", Weight: 1.0}, // weak bridge
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.01
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on ultra-large weights: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if math.IsNaN(res.Quality) || math.IsInf(res.Quality, 0) {
			t.Errorf("Quality is non-finite: %f", res.Quality)
		}
	})

	// Scenario B: Ultra-small weights (1e-12)
	t.Run("UltraSmallWeights", func(t *testing.T) {
		edges := []RawEdge{
			{SourceID: "A", TargetID: "B", Type: "CALLS", Weight: 1e-12},
			{SourceID: "B", TargetID: "C", Type: "CALLS", Weight: 1e-12},
			{SourceID: "D", TargetID: "E", Type: "CALLS", Weight: 1e-12},
		}

		cfg := DefaultConfig()
		cfg.Gamma = 1e-15
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on ultra-small weights: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
		if math.IsNaN(res.Quality) {
			t.Errorf("Quality is NaN: %f", res.Quality)
		}
	})

	// Scenario C: Diverse edge types with custom matrix multipliers
	t.Run("CustomEdgeTypeMatrix", func(t *testing.T) {
		customMatrix := EdgeWeightMatrix{
			CallsWeight:            2.5,
			ContainsWeight:         1.8,
			InheritsWeight:         3.0,
			UsesGlobalWeight:       0.2,
			CoChangedWeight:        1.5,
			ReferencesWeight:       0.1,
			ImplicitSemanticWeight: 0.9,
		}

		edges := []RawEdge{
			{SourceID: "A", TargetID: "B", Type: "CALLS", Weight: 1.0},
			{SourceID: "B", TargetID: "C", Type: "CONTAINS", Weight: 1.0},
			{SourceID: "C", TargetID: "A", Type: "INHERITS", Weight: 1.0},
			{SourceID: "D", TargetID: "E", Type: "CO_CHANGED", Weight: 5.0},
			{SourceID: "E", TargetID: "F", Type: "USES_GLOBAL", Weight: 1.0},
			{SourceID: "A", TargetID: "D", Type: "TESTS", Weight: 10.0},             // Should be ignored (0.0)
			{SourceID: "B", TargetID: "E", Type: "IMPLICIT_SEMANTIC", Weight: 0.90}, // Valid affinity
			{SourceID: "C", TargetID: "F", Type: "IMPLICIT_SEMANTIC", Weight: 0.50}, // Sub-threshold -> ignored
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.05
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, customMatrix)

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed on custom matrix: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
	})

	// Scenario D: Multi-edge consolidation stress (100 parallel edges between A and B)
	t.Run("MultiEdgeConsolidation", func(t *testing.T) {
		var multiEdges []RawEdge
		for i := 0; i < 100; i++ {
			multiEdges = append(multiEdges, RawEdge{
				SourceID: "A",
				TargetID: "B",
				Type:     "CALLS",
				Weight:   0.5,
			})
		}
		multiEdges = append(multiEdges, RawEdge{SourceID: "C", TargetID: "D", Type: "CALLS", Weight: 1.0})

		cfg := DefaultConfig()
		cfg.Gamma = 0.1
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, multiEdges)
		if err != nil {
			t.Fatalf("Partition failed on multi-edge consolidation: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)
	})
}

// 3. Extreme Graph Topologies
func TestAdversarial_GraphTopologies(t *testing.T) {
	// Topology A: Star Graph S_101 (Center node c0 connected to 100 leaf nodes)
	t.Run("StarGraph_S101", func(t *testing.T) {
		starNodes := make([]string, 101)
		starNodes[0] = "center"
		for i := 1; i <= 100; i++ {
			starNodes[i] = fmt.Sprintf("leaf_%d", i)
		}

		var starEdges []RawEdge
		for i := 1; i <= 100; i++ {
			starEdges = append(starEdges, RawEdge{
				SourceID: "center",
				TargetID: fmt.Sprintf("leaf_%d", i),
				Type:     "CALLS",
			})
		}

		cfg := DefaultConfig()
		cfg.SuppressHubs = true
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(starNodes, starEdges)
		if err != nil {
			t.Fatalf("Star graph partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, starNodes, res)

		// Hub quarantine should isolate 'center'
		var hubFound bool
		for _, h := range res.CrossCuttingHubs {
			if h.NodeID == "center" {
				hubFound = true
				if h.Degree != 100 {
					t.Errorf("Expected hub degree 100, got %d", h.Degree)
				}
			}
		}
		if !hubFound {
			t.Errorf("Expected 'center' to be quarantined as hub in star graph")
		}
	})

	// Topology B: Long Linear Chain P_200 (200 nodes in a single line)
	t.Run("LinearPath_P200", func(t *testing.T) {
		pathNodes := make([]string, 200)
		for i := 0; i < 200; i++ {
			pathNodes[i] = fmt.Sprintf("p_%d", i)
		}
		var pathEdges []RawEdge
		for i := 0; i < 199; i++ {
			pathEdges = append(pathEdges, RawEdge{
				SourceID: fmt.Sprintf("p_%d", i),
				TargetID: fmt.Sprintf("p_%d", i+1),
				Type:     "CALLS",
			})
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.05
		cfg.SuppressHubs = true
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(pathNodes, pathEdges)
		if err != nil {
			t.Fatalf("Linear path partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, pathNodes, res)
	})

	// Topology C: Cycle Graph C_150 (150 nodes in a ring)
	t.Run("CycleRing_C150", func(t *testing.T) {
		cycleNodes := make([]string, 150)
		for i := 0; i < 150; i++ {
			cycleNodes[i] = fmt.Sprintf("c_%d", i)
		}
		var cycleEdges []RawEdge
		for i := 0; i < 150; i++ {
			cycleEdges = append(cycleEdges, RawEdge{
				SourceID: fmt.Sprintf("c_%d", i),
				TargetID: fmt.Sprintf("c_%d", (i+1)%150),
				Type:     "CALLS",
			})
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.05
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(cycleNodes, cycleEdges)
		if err != nil {
			t.Fatalf("Cycle ring partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, cycleNodes, res)
	})

	// Topology D: Complete Binary Tree (127 nodes, depth 7)
	t.Run("CompleteBinaryTree_127", func(t *testing.T) {
		numNodes := 127
		treeNodes := make([]string, numNodes)
		for i := 0; i < numNodes; i++ {
			treeNodes[i] = fmt.Sprintf("t_%d", i)
		}
		var treeEdges []RawEdge
		for i := 0; i < numNodes/2; i++ {
			left := 2*i + 1
			right := 2*i + 2
			if left < numNodes {
				treeEdges = append(treeEdges, RawEdge{
					SourceID: fmt.Sprintf("t_%d", i),
					TargetID: fmt.Sprintf("t_%d", left),
					Type:     "CALLS",
				})
			}
			if right < numNodes {
				treeEdges = append(treeEdges, RawEdge{
					SourceID: fmt.Sprintf("t_%d", i),
					TargetID: fmt.Sprintf("t_%d", right),
					Type:     "CALLS",
				})
			}
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.02
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(treeNodes, treeEdges)
		if err != nil {
			t.Fatalf("Binary tree partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, treeNodes, res)
	})

	// Topology E: Sparse Bipartite Graph K_{50, 50} with 5% edge density
	t.Run("SparseBipartite_K50_50", func(t *testing.T) {
		bipartiteNodes := make([]string, 100)
		for i := 0; i < 50; i++ {
			bipartiteNodes[i] = fmt.Sprintf("left_%d", i)
			bipartiteNodes[50+i] = fmt.Sprintf("right_%d", i)
		}

		var bipartiteEdges []RawEdge
		rng := rand.New(rand.NewSource(999))
		for i := 0; i < 50; i++ {
			for j := 0; j < 50; j++ {
				if rng.Float64() < 0.05 {
					bipartiteEdges = append(bipartiteEdges, RawEdge{
						SourceID: fmt.Sprintf("left_%d", i),
						TargetID: fmt.Sprintf("right_%d", j),
						Type:     "CALLS",
					})
				}
			}
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.01
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(bipartiteNodes, bipartiteEdges)
		if err != nil {
			t.Fatalf("Sparse bipartite partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, bipartiteNodes, res)
	})

	// Topology F: Disconnected Forest (10 separate trees of 10 nodes each)
	t.Run("DisconnectedForest_10x10", func(t *testing.T) {
		var forestNodes []string
		var forestEdges []RawEdge

		for treeIdx := 0; treeIdx < 10; treeIdx++ {
			for nodeIdx := 0; nodeIdx < 10; nodeIdx++ {
				id := fmt.Sprintf("tree%d_n%d", treeIdx, nodeIdx)
				forestNodes = append(forestNodes, id)
				if nodeIdx > 0 {
					parent := fmt.Sprintf("tree%d_n%d", treeIdx, (nodeIdx-1)/2)
					forestEdges = append(forestEdges, RawEdge{
						SourceID: parent,
						TargetID: id,
						Type:     "CALLS",
					})
				}
			}
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.01
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(forestNodes, forestEdges)
		if err != nil {
			t.Fatalf("Disconnected forest partition failed: %v", err)
		}
		verifyPartitionIntegrity(t, forestNodes, res)
	})
}

// 4. Recursive Hierarchical Sub-Clustering Depth and Limits
func TestAdversarial_RecursiveSubClusteringDepth(t *testing.T) {
	// Synthesize a huge monolithic community of 300 nodes consisting of 6 tightly coupled sub-cliques of 50 nodes
	// each with high internal density and weak inter-clique coupling.
	totalNodes := 300
	nodes := make([]string, totalNodes)
	for i := 0; i < totalNodes; i++ {
		nodes[i] = fmt.Sprintf("node_%03d", i)
	}

	var edges []RawEdge
	// 6 sub-cliques of 50 nodes
	for c := 0; c < 6; c++ {
		start := c * 50
		for i := start; i < start+50; i++ {
			for j := i + 1; j < start+50; j++ {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("node_%03d", i),
					TargetID: fmt.Sprintf("node_%03d", j),
					Type:     "CALLS",
				})
			}
		}
	}
	// Weak inter-clique ring coupling
	for c := 0; c < 6; c++ {
		next := (c + 1) % 6
		edges = append(edges, RawEdge{
			SourceID: fmt.Sprintf("node_%03d", c*50+49),
			TargetID: fmt.Sprintf("node_%03d", next*50),
			Type:     "CALLS",
		})
	}

	// Test A: If Gamma is artificially low (0.0001), initial Leiden produces 1 big community of 300 nodes (> MaxCommunitySize 100).
	// Sub-clustering should split it into smaller communities.
	t.Run("RecursiveSplitOversized", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Gamma = 0.0001
		cfg.MinCommunitySize = 20
		cfg.MaxCommunitySize = 100
		cfg.MaxHierDepth = 3
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Partition failed during recursive sub-clustering: %v", err)
		}
		verifyPartitionIntegrity(t, nodes, res)

		// Ensure that all communities satisfy or are split towards MaxCommunitySize
		for _, comm := range res.Communities {
			if comm.Size > 100 {
				t.Errorf("Community %s has size %d > MaxCommunitySize (100)", comm.ID, comm.Size)
			}
		}
		if len(res.Communities) < 3 {
			t.Errorf("Expected at least 3 sub-clustered communities, got %d", len(res.Communities))
		}
	})

	// Test B: Indivisible dense clique of 200 nodes where internal edges cannot be split even at higher gamma.
	// Must gracefully terminate when depth reaches MaxHierDepth without infinite loop.
	t.Run("IndivisibleDenseCliqueDepthTermination", func(t *testing.T) {
		cliqueNodes := make([]string, 150)
		for i := 0; i < 150; i++ {
			cliqueNodes[i] = fmt.Sprintf("k150_%d", i)
		}
		var cliqueEdges []RawEdge
		for i := 0; i < 150; i++ {
			for j := i + 1; j < 150; j++ {
				cliqueEdges = append(cliqueEdges, RawEdge{
					SourceID: fmt.Sprintf("k150_%d", i),
					TargetID: fmt.Sprintf("k150_%d", j),
					Type:     "CALLS",
				})
			}
		}

		cfg := DefaultConfig()
		cfg.Gamma = 0.001
		cfg.MaxCommunitySize = 50 // artificially small threshold
		cfg.MaxHierDepth = 2      // max depth limit
		cfg.SuppressHubs = false
		engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

		res, err := engine.Partition(cliqueNodes, cliqueEdges)
		if err != nil {
			t.Fatalf("Indivisible clique sub-clustering failed: %v", err)
		}
		verifyPartitionIntegrity(t, cliqueNodes, res)
	})
}

// 5. Boundary Participation Ratio (BPR) Threshold & Accuracy Probes
func TestAdversarial_BPRBoundaryThresholds(t *testing.T) {
	// Construct 2 isolated communities C1 (10 nodes) and C2 (10 nodes)
	// Place boundary test nodes with precisely controlled incident weights
	nodes := []string{
		"boundary_exact_25",  // 25% to C1, 75% to C2 (>= 0.25 on both? 0.25 on C1, 0.75 on C2 -> count = 2 -> SharedBoundary)
		"boundary_below_249", // 24% to C1, 76% to C2 (C1 is < 0.25 -> count = 1 -> NOT SharedBoundary)
		"boundary_above_251", // 26% to C1, 74% to C2 (both >= 0.25 -> count = 2 -> SharedBoundary)
		"boundary_equal_50",  // 50% to C1, 50% to C2 (count = 2 -> SharedBoundary)
		"isolated_node",      // 0 edges (should not panic)
	}

	for i := 0; i < 10; i++ {
		nodes = append(nodes, fmt.Sprintf("c1_%d", i))
		nodes = append(nodes, fmt.Sprintf("c2_%d", i))
	}

	var edges []RawEdge
	// Internal C1 edges (all-to-all)
	for i := 0; i < 10; i++ {
		for j := i + 1; j < 10; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("c1_%d", i), TargetID: fmt.Sprintf("c1_%d", j), Type: "CALLS"})
		}
	}
	// Internal C2 edges (all-to-all)
	for i := 0; i < 10; i++ {
		for j := i + 1; j < 10; j++ {
			edges = append(edges, RawEdge{SourceID: fmt.Sprintf("c2_%d", i), TargetID: fmt.Sprintf("c2_%d", j), Type: "CALLS"})
		}
	}

	// Connect boundary test nodes
	// 1. boundary_exact_25: 1 edge to c1_0 (weight 1.0), 3 edges to c2_0, c2_1, c2_2 (weights 1.0 each -> total 4.0, ratio C1 = 1/4 = 0.25, C2 = 3/4 = 0.75)
	edges = append(edges, RawEdge{SourceID: "boundary_exact_25", TargetID: "c1_0", Type: "CALLS", Weight: 1.0})
	edges = append(edges, RawEdge{SourceID: "boundary_exact_25", TargetID: "c2_0", Type: "CALLS", Weight: 1.0})
	edges = append(edges, RawEdge{SourceID: "boundary_exact_25", TargetID: "c2_1", Type: "CALLS", Weight: 1.0})
	edges = append(edges, RawEdge{SourceID: "boundary_exact_25", TargetID: "c2_2", Type: "CALLS", Weight: 1.0})

	// 2. boundary_below_249: 24 edges weight equivalent to C1, 76 to C2
	edges = append(edges, RawEdge{SourceID: "boundary_below_249", TargetID: "c1_0", Type: "CALLS", Weight: 0.24})
	edges = append(edges, RawEdge{SourceID: "boundary_below_249", TargetID: "c2_0", Type: "CALLS", Weight: 0.76})

	// 3. boundary_above_251: 26 edges weight equivalent to C1, 74 to C2
	edges = append(edges, RawEdge{SourceID: "boundary_above_251", TargetID: "c1_0", Type: "CALLS", Weight: 0.26})
	edges = append(edges, RawEdge{SourceID: "boundary_above_251", TargetID: "c2_0", Type: "CALLS", Weight: 0.74})

	// 4. boundary_equal_50: 50% to C1, 50% to C2
	edges = append(edges, RawEdge{SourceID: "boundary_equal_50", TargetID: "c1_0", Type: "CALLS", Weight: 1.0})
	edges = append(edges, RawEdge{SourceID: "boundary_equal_50", TargetID: "c2_0", Type: "CALLS", Weight: 1.0})

	cfg := DefaultConfig()
	cfg.Gamma = 0.1
	cfg.SuppressHubs = false
	engine := NewEngine(cfg, DefaultEdgeWeightMatrix())

	res, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("BPR boundary partition failed: %v", err)
	}
	verifyPartitionIntegrity(t, nodes, res)

	boundaryFlags := make(map[string]bool)
	for _, sb := range res.SharedBoundaries {
		boundaryFlags[sb.NodeID] = true
	}

	if !boundaryFlags["boundary_exact_25"] {
		t.Errorf("Expected boundary_exact_25 (BPR=0.25) to be flagged as SharedBoundaryNode")
	}
	if boundaryFlags["boundary_below_249"] {
		t.Errorf("Expected boundary_below_249 (BPR=0.24) NOT to be flagged as SharedBoundaryNode")
	}
	if !boundaryFlags["boundary_above_251"] {
		t.Errorf("Expected boundary_above_251 (BPR=0.26) to be flagged as SharedBoundaryNode")
	}
	if !boundaryFlags["boundary_equal_50"] {
		t.Errorf("Expected boundary_equal_50 (BPR=0.50) to be flagged as SharedBoundaryNode")
	}
	if boundaryFlags["isolated_node"] {
		t.Errorf("Expected isolated_node NOT to be flagged as SharedBoundaryNode")
	}
}

// 6. Stress Determinism across 50 Repeated Runs on Irregular Graphs
func TestAdversarial_Stress50RunDeterminism(t *testing.T) {
	n := 120
	nodes := make([]string, n)
	for i := 0; i < n; i++ {
		nodes[i] = fmt.Sprintf("node_%03d", i)
	}

	var edges []RawEdge
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Pseudorandom pseudo-chaotic deterministic hash edge generator
			h := (i*37 + j*73 + i*j*19) % 23
			if h < 3 {
				edges = append(edges, RawEdge{
					SourceID: fmt.Sprintf("node_%03d", i),
					TargetID: fmt.Sprintf("node_%03d", j),
					Type:     "CALLS",
					Weight:   float64((h + 1)) * 0.5,
				})
			}
		}
	}

	cfg := DefaultConfig()
	cfg.RandomSeed = 88888
	cfg.SuppressHubs = true

	var baselineCommNodes [][]string
	var baselineQuality float64

	for run := 0; run < 50; run++ {
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
			baselineQuality = res.Quality
		} else {
			if !reflect.DeepEqual(baselineCommNodes, currentCommNodes) {
				t.Fatalf("Non-determinism detected on run %d! Partition differed from baseline.", run)
			}
			if math.Abs(baselineQuality-res.Quality) > 1e-9 {
				t.Fatalf("Quality non-determinism on run %d: expected %f, got %f", run, baselineQuality, res.Quality)
			}
		}
	}
}
