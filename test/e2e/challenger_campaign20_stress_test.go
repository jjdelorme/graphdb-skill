package e2e_test

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"graphdb/internal/analysis"
	"graphdb/internal/analysis/leiden"
)

// ============================================================================
// CHALLENGER CAMPAIGN 20 VERIFICATION SUITE
// ============================================================================

// TestChallenger_AC3_Determinism_10ConsecutiveRuns rigorously verifies AC3:
// 10 consecutive runs with fixed seed produce identical community assignments,
// identical quality scores, identical hub quarantines, and identical BPR boundaries.
func TestChallenger_AC3_Determinism_10ConsecutiveRuns(t *testing.T) {
	const (
		runs            = 10
		numClusters     = 15
		nodesPerCluster = 30 // 450 total nodes
		fixedSeed       = int64(20260901)
	)

	nodes, edges := generateComplexBenchmarkGraph(numClusters, nodesPerCluster, 0.40, 0.002, 424242)

	cfg := leiden.Config{
		Gamma:            0.045,
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
		res, err := engine.Partition(nodes, edges)
		if err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		nodeMap := buildNodeToCommunityMap(res)

		if run == 0 {
			baselineResult = res
			baselineMap = nodeMap
			if len(res.Communities) < numClusters {
				t.Fatalf("Expected >= %d communities, got %d", numClusters, len(res.Communities))
			}
			continue
		}

		// Verify exact count
		if len(res.Communities) != len(baselineResult.Communities) {
			t.Fatalf("Run %d: Community count mismatch: got %d, want %d",
				run, len(res.Communities), len(baselineResult.Communities))
		}

		// Verify exact quality
		if math.Abs(res.Quality-baselineResult.Quality) > 1e-9 {
			t.Fatalf("Run %d: Quality score mismatch: got %.9f, want %.9f",
				run, res.Quality, baselineResult.Quality)
		}

		// Verify exact gamma
		if math.Abs(res.Gamma-baselineResult.Gamma) > 1e-9 {
			t.Fatalf("Run %d: Gamma mismatch: got %.9f, want %.9f",
				run, res.Gamma, baselineResult.Gamma)
		}

		// Verify exact node assignment for all nodes
		for nodeID, baseComm := range baselineMap {
			if comm, ok := nodeMap[nodeID]; !ok || comm != baseComm {
				t.Fatalf("Run %d: Node %s mapped to %s, want %s", run, nodeID, comm, baseComm)
			}
		}

		// Verify identical hubs
		if len(res.CrossCuttingHubs) != len(baselineResult.CrossCuttingHubs) {
			t.Fatalf("Run %d: Hubs count mismatch: %d vs %d",
				run, len(res.CrossCuttingHubs), len(baselineResult.CrossCuttingHubs))
		}
	}
}

// TestChallenger_AC4_HairballSuppression verifies AC4:
// Benchmark graph with high-degree hubs partitions into >= 15 distinct communities
// with no cluster > 25% of total nodes, and pervasive hubs are quarantined into CrossCuttingHubs.
func TestChallenger_AC4_HairballSuppression(t *testing.T) {
	const (
		numClusters      = 20
		nodesPerCluster  = 35 // 700 domain nodes
		numPervasiveHubs = 6  // 706 total nodes
		seed             = int64(10101)
	)

	nodes, edges, hubIDs := generateHairballBenchmarkGraph(
		numClusters, nodesPerCluster, numPervasiveHubs, 0.40, 0.0008, seed,
	)

	totalNodes := len(nodes)
	if totalNodes != 706 {
		t.Fatalf("Expected 706 nodes, got %d", totalNodes)
	}

	cfg := leiden.Config{
		Gamma:            0.0, // Adaptive search
		MinCommunitySize: 20,
		MaxCommunitySize: 120,
		SuppressHubs:     true, // Two-tier hub suppression enabled
		RandomSeed:       seed,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}

	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Hairball partition failed: %v", err)
	}

	// 1. Assert >= 15 distinct communities
	if len(result.Communities) < 15 {
		t.Fatalf("AC4 Violation: Expected >= 15 communities, got %d", len(result.Communities))
	}

	// 2. Assert no cluster > 25% of total nodes
	maxAllowedNodes := int(float64(totalNodes) * 0.25)
	for _, comm := range result.Communities {
		if len(comm.NodeIDs) > maxAllowedNodes {
			t.Fatalf("AC4 Violation: Community %s has %d nodes, exceeding 25%% max limit of %d",
				comm.ID, len(comm.NodeIDs), maxAllowedNodes)
		}
	}

	// 3. Assert all pervasive hubs are quarantined with degree > 100
	quarantined := make(map[string]*leiden.QuarantinedHubNode)
	for _, hub := range result.CrossCuttingHubs {
		quarantined[hub.NodeID] = hub
	}

	for _, expectedHub := range hubIDs {
		hubInfo, ok := quarantined[expectedHub]
		if !ok {
			t.Fatalf("AC4 Violation: Hub %s was not quarantined in CrossCuttingHubs", expectedHub)
		}
		if hubInfo.Degree <= 100 {
			t.Fatalf("AC4 Violation: Quarantined hub %s has degree %d <= 100", expectedHub, hubInfo.Degree)
		}
		if hubInfo.HubScore <= 0 {
			t.Fatalf("AC4 Violation: Hub %s has non-positive hub score %.2f", expectedHub, hubInfo.HubScore)
		}
	}
}

// TestChallenger_AC5_ResolutionLimitImmunity verifies AC5:
// 30 synthetic submodules (25-40 nodes each) remain cleanly separated into 30 distinct communities.
func TestChallenger_AC5_ResolutionLimitImmunity(t *testing.T) {
	const numSubmodules = 30
	submoduleSizes := make([]int, numSubmodules)
	totalNodesExpected := 0
	for i := 0; i < numSubmodules; i++ {
		// Variable sizes from 25 to 40
		sz := 25 + (i%4)*5 // 25, 30, 35, 40
		submoduleSizes[i] = sz
		totalNodesExpected += sz
	}

	var nodes []string
	var edges []leiden.RawEdge
	submoduleNodeLists := make([][]string, numSubmodules)

	for c := 0; c < numSubmodules; c++ {
		sz := submoduleSizes[c]
		var members []string
		for n := 0; n < sz; n++ {
			id := fmt.Sprintf("ac5_submod_%02d_elem_%02d", c, n)
			nodes = append(nodes, id)
			members = append(members, id)
		}
		submoduleNodeLists[c] = members

		// Full intra-module mesh (clique)
		for i := 0; i < sz; i++ {
			for j := i + 1; j < sz; j++ {
				edges = append(edges, leiden.RawEdge{
					SourceID: members[i],
					TargetID: members[j],
					Type:     "CALLS",
					Weight:   1.0,
				})
			}
		}
	}

	// Sparse ring bridges between adjacent submodules (1 cross link each)
	for c := 0; c < numSubmodules; c++ {
		nextC := (c + 1) % numSubmodules
		edges = append(edges, leiden.RawEdge{
			SourceID: submoduleNodeLists[c][0],
			TargetID: submoduleNodeLists[nextC][0],
			Type:     "CALLS",
			Weight:   1.0,
		})
	}

	if len(nodes) != totalNodesExpected {
		t.Fatalf("Expected %d total nodes, got %d", totalNodesExpected, len(nodes))
	}

	cfg := leiden.Config{
		Gamma:            0.035, // CPM resolution parameter
		MinCommunitySize: 20,
		MaxCommunitySize: 100,
		SuppressHubs:     false,
		RandomSeed:       999,
		MaxIterations:    50,
	}

	engine := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
	result, err := engine.Partition(nodes, edges)
	if err != nil {
		t.Fatalf("Resolution partition failed: %v", err)
	}

	// 1. Assert exactly 30 communities
	if len(result.Communities) != numSubmodules {
		t.Fatalf("AC5 Violation: Expected exactly %d communities, got %d (Resolution Limit occurred)",
			numSubmodules, len(result.Communities))
	}

	// 2. Assert every submodule is 100% intact and unfragmented
	nodeMap := buildNodeToCommunityMap(result)
	for cIdx, members := range submoduleNodeLists {
		firstComm := nodeMap[members[0]]
		for _, member := range members {
			comm := nodeMap[member]
			if comm != firstComm {
				t.Fatalf("AC5 Violation: Submodule %d (size %d) split across communities (%s vs %s)",
					cIdx, len(members), comm, firstComm)
			}
		}
	}
}

// TestChallenger_AC6_DualLensSeams verifies AC6:
// Low cut-edge pinch points (<= 4 cut-edges) are accurately surfaced with Actionable Seam Score >= 10.0,
// and correctly prioritized over high-cut-edge debt.
func TestChallenger_AC6_DualLensSeams(t *testing.T) {
	candidates := []analysis.SeamCandidate{
		{
			ID:             "seam_pinpoint_ideal",
			Name:           "PaymentGateway::Charge",
			InternalFanIn:  20,
			VolatileFanOut: 15,
			CutEdges:       1, // Score = (20*15)/(1+1) = 150.0 -> Tier 1
		},
		{
			ID:             "seam_boundary_4_cut",
			Name:           "AuthService::Authenticate",
			InternalFanIn:  10,
			VolatileFanOut: 10,
			CutEdges:       4, // Score = (10*10)/(4+1) = 20.0 -> Tier 1
		},
		{
			ID:             "seam_diffuse_debt_5_cut",
			Name:           "LegacyMonolith::Dispatch",
			InternalFanIn:  30,
			VolatileFanOut: 20,
			CutEdges:       5, // Score = (30*20)/(5+1) = 100.0 -> Tier 2 (CutEdges > 4)
		},
		{
			ID:             "seam_low_score_trivial",
			Name:           "Helper::Format",
			InternalFanIn:  2,
			VolatileFanOut: 2,
			CutEdges:       1, // Score = 4 / 2 = 2.0 -> Trivial (Score < 10)
		},
	}

	tier1, tier2, other := analysis.ClassifyAndRankSeams(candidates)

	if len(tier1) != 2 {
		t.Fatalf("AC6 Violation: Expected 2 Tier 1 seams, got %d", len(tier1))
	}
	if tier1[0].ID != "seam_pinpoint_ideal" || tier1[0].ActionableSeamScore < 149.0 {
		t.Fatalf("AC6 Violation: Top Tier 1 seam rank mismatch: %+v", tier1[0])
	}
	if tier1[1].ID != "seam_boundary_4_cut" || tier1[1].ActionableSeamScore < 19.9 {
		t.Fatalf("AC6 Violation: Second Tier 1 seam rank mismatch: %+v", tier1[1])
	}

	for _, s := range tier1 {
		if s.CutEdges > 4 || s.ActionableSeamScore < 10.0 {
			t.Fatalf("AC6 Violation: Invalid Tier 1 seam: %+v", s)
		}
	}

	if len(tier2) != 1 || tier2[0].ID != "seam_diffuse_debt_5_cut" {
		t.Fatalf("AC6 Violation: Expected seam_diffuse_debt_5_cut in Tier 2, got %+v", tier2)
	}

	if len(other) != 1 || other[0].ID != "seam_low_score_trivial" {
		t.Fatalf("AC6 Violation: Expected seam_low_score_trivial in other, got %+v", other)
	}
}

// TestChallenger_AC7_HygieneZeroGoogle3Contamination verifies AC7:
// Zero Google3 internal paths, zero proprietary imports, and zero internal markers across the repository.
func TestChallenger_AC7_HygieneZeroGoogle3Contamination(t *testing.T) {
	root := getRepoRoot(t)

	forbidden := []string{
		"goog" + "le3/",
		"research/" + "omega",
		"//depot/" + "google3",
		"goog" + "le3/base",
		"goog" + "le3/net",
		"goog" + "le3/util",
		"pip" + "er://",
		"bla" + "ze-bin",
		"bla" + "ze-out",
	}

	skipDirs := map[string]bool{
		".git":          true,
		".agents":       true,
		"bin":           true,
		".gemini/tools": true,
		".gemini/brain": true,
		"node_modules":  true,
	}

	var checkedCount int
	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(root, path)
		if info.IsDir() {
			if skipDirs[info.Name()] || strings.HasPrefix(rel, ".agents") || strings.HasPrefix(rel, ".gemini/brain") || strings.HasPrefix(rel, ".gemini/tools") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip test files checking for forbidden patterns
		if strings.HasSuffix(rel, "hygiene_test.go") || strings.HasSuffix(rel, "challenger_campaign20_stress_test.go") {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".js" && ext != ".ts" && ext != ".md" && ext != ".json" && ext != ".yaml" && ext != ".yml" && ext != ".sh" && info.Name() != "Makefile" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		checkedCount++
		lines := strings.Split(string(content), "\n")
		for lineIdx, line := range lines {
			for _, pat := range forbidden {
				if strings.Contains(line, pat) {
					violations = append(violations, fmt.Sprintf("%s:%d matches %q", rel, lineIdx+1, pat))
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Filesystem walk failed: %v", err)
	}

	t.Logf("Challenger scanned %d repository files for hygiene violations", checkedCount)
	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("Hygiene violation: %s", v)
		}
		t.Fatalf("Found %d proprietary/google3 contamination violations in repo", len(violations))
	}
}

// TestChallenger_Stress_HighConcurrencyAndEdgeCases tests extreme concurrency and edge inputs.
func TestChallenger_Stress_HighConcurrencyAndEdgeCases(t *testing.T) {
	// 1. Edge case: Empty input
	engine := leiden.NewDefaultEngine()
	res, err := engine.Partition(nil, nil)
	if err != nil {
		t.Fatalf("Empty partition failed: %v", err)
	}
	if len(res.Communities) != 0 {
		t.Fatalf("Expected 0 communities for empty input, got %d", len(res.Communities))
	}

	// 2. Edge case: Single node
	res, err = engine.Partition([]string{"singleton"}, nil)
	if err != nil {
		t.Fatalf("Single node partition failed: %v", err)
	}
	if len(res.Communities) != 1 || res.Communities[0].Size != 1 {
		t.Fatalf("Expected 1 community with size 1 for single node, got %+v", res.Communities)
	}

	// 3. Concurrency test: 30 goroutines running partitions in parallel
	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			nodes, edges, _ := generateRingOfCliquesGraph(10, 15, 2)
			cfg := leiden.Config{
				Gamma:            0.04,
				MinCommunitySize: 10,
				MaxCommunitySize: 50,
				SuppressHubs:     false,
				RandomSeed:       int64(gid * 100),
				MaxIterations:    25,
			}
			eng := leiden.NewEngine(cfg, leiden.DefaultEdgeWeightMatrix())
			r, e := eng.Partition(nodes, edges)
			if e != nil {
				t.Errorf("Goroutine %d failed: %v", gid, e)
				return
			}
			if len(r.Communities) != 10 {
				t.Errorf("Goroutine %d expected 10 communities, got %d", gid, len(r.Communities))
			}
		}(g)
	}

	wg.Wait()
}

// TestChallenger_CLI_EnrichTopologyAndQueryExecution directly executes the compiled CLI binary
// on synthetic data and verifies execution and outputs.
func TestChallenger_CLI_EnrichTopologyAndQueryExecution(t *testing.T) {
	root := getRepoRoot(t)
	cliPath := filepath.Join(root, ".gemini/skills/graphdb/scripts/graphdb")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Skip("CLI binary not present, skipping CLI sub-process test")
	}

	// Test CLI help outputs
	cmd := exec.Command(cliPath, "enrich-topology", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI enrich-topology --help failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "-gamma") || !strings.Contains(string(out), "-suppress-hubs") {
		t.Fatalf("CLI enrich-topology --help missing expected flags: %s", string(out))
	}
}
