package leiden

import (
	"math"
	"math/rand"
	"sort"
)

// EvaluatePenalty computes the size-based penalty score for a given partition:
// Phi(P) = 0.35 * FracSmall + 0.45 * FracLarge + 0.20 * (|C_max| / |V|)
func EvaluatePenalty(partition []int, minSize int, maxSize int) float64 {
	n := len(partition)
	if n == 0 {
		return 0.0
	}

	commSizes := make(map[int]int)
	for _, c := range partition {
		commSizes[c]++
	}

	var smallNodes, largeNodes, maxNodes int
	for _, sz := range commSizes {
		if sz < minSize {
			smallNodes += sz
		}
		if sz > maxSize {
			largeNodes += sz
		}
		if sz > maxNodes {
			maxNodes = sz
		}
	}

	nFloat := float64(n)
	fracSmall := float64(smallNodes) / nFloat
	fracLarge := float64(largeNodes) / nFloat
	fracMax := float64(maxNodes) / nFloat

	return 0.35*fracSmall + 0.45*fracLarge + 0.20*fracMax
}

// computeMedianSize computes the median size across all communities in a partition.
func computeMedianSize(partition []int) float64 {
	if len(partition) == 0 {
		return 0.0
	}

	commSizes := make(map[int]int)
	for _, c := range partition {
		commSizes[c]++
	}

	sizes := make([]int, 0, len(commSizes))
	for _, sz := range commSizes {
		sizes = append(sizes, sz)
	}
	sort.Ints(sizes)

	m := len(sizes)
	if m == 0 {
		return 0.0
	}
	if m%2 == 1 {
		return float64(sizes[m/2])
	}
	return float64(sizes[m/2-1]+sizes[m/2]) / 2.0
}

// SearchOptimalGamma performs dynamic bisection search over the CPM resolution parameter gamma.
func SearchOptimalGamma(g *Graph, cfg Config, seed int64) (float64, []int) {
	n := len(g.NodeIDs)
	if n == 0 {
		return 0.01, nil
	}
	if n == 1 {
		return 0.01, []int{0}
	}

	// Calculate average effective edge density: rho = 2 * sum(W_eff) / (|V| * (|V| - 1))
	var rho float64
	if n >= 2 {
		rho = (2.0 * g.TotalWeight) / (float64(n) * float64(n-1))
	}
	if rho <= 0 {
		rho = 0.01
	}

	gammaLow := 0.05 * rho
	if gammaLow < 1e-5 {
		gammaLow = 1e-5
	}
	gammaHigh := 20.0 * rho
	if gammaHigh > 1.0 {
		gammaHigh = 1.0
	}
	if gammaHigh <= gammaLow {
		gammaHigh = gammaLow * 100.0
	}

	steps := cfg.ResolutionSteps
	if steps <= 0 {
		steps = 8
	}

	bestGamma := gammaLow
	bestScore := math.Inf(1)
	var bestPartition []int

	for step := 0; step < steps; step++ {
		gammaMid := math.Sqrt(gammaLow * gammaHigh)
		stepRNG := rand.New(rand.NewSource(seed + int64(step*1000)))

		pMid := LeidenClustering(g, gammaMid, cfg.MaxIterations, stepRNG)
		score := EvaluatePenalty(pMid, cfg.MinCommunitySize, cfg.MaxCommunitySize)

		if score < bestScore {
			bestScore = score
			bestGamma = gammaMid
			bestPartition = pMid
		}

		medSize := computeMedianSize(pMid)
		if medSize > float64(cfg.MaxCommunitySize) {
			// Communities too large -> increase density threshold
			gammaLow = gammaMid
		} else if medSize < float64(cfg.MinCommunitySize) {
			// Communities too small -> decrease density threshold
			gammaHigh = gammaMid
		} else {
			// In ideal range -> narrow search around gammaMid
			gammaLow = gammaMid * 0.7
			gammaHigh = gammaMid * 1.4
		}

		if gammaHigh <= gammaLow {
			break
		}
	}

	if bestPartition == nil {
		bestPartition = LeidenClustering(g, bestGamma, cfg.MaxIterations, rand.New(rand.NewSource(seed)))
	}

	return bestGamma, bestPartition
}

// SubClusterOversized recursively partitions communities exceeding MaxCommunitySize.
func SubClusterOversized(
	g *Graph,
	partition []int,
	parentGamma float64,
	cfg Config,
	seed int64,
	currentDepth int,
) []int {
	if currentDepth >= cfg.MaxHierDepth {
		return partition
	}

	n := len(partition)
	commMembers := make(map[int][]int)
	for i, c := range partition {
		commMembers[c] = append(commMembers[c], i)
	}

	// Sort community keys for deterministic iteration order
	commKeys := make([]int, 0, len(commMembers))
	for c := range commMembers {
		commKeys = append(commKeys, c)
	}
	sort.Ints(commKeys)

	finalPartition := make([]int, n)
	copy(finalPartition, partition)
	nextFreshCommID := 0
	for _, c := range partition {
		if c >= nextFreshCommID {
			nextFreshCommID = c + 1
		}
	}

	for _, commID := range commKeys {
		members := commMembers[commID]
		if len(members) <= cfg.MaxCommunitySize {
			continue
		}

		// Extract induced subgraph for members of this oversized community
		subGraph, memberToOrig := extractInducedSubgraph(g, members)
		if len(subGraph.NodeIDs) <= cfg.MaxCommunitySize {
			continue
		}

		localGamma := 1.5 * parentGamma
		subRNG := rand.New(rand.NewSource(seed + int64(currentDepth*7919) + int64(commID*101)))
		subPart := LeidenClustering(subGraph, localGamma, cfg.MaxIterations, subRNG)

		// Check if sub-clustering split into multiple communities
		subUnique := make(map[int]struct{})
		for _, sc := range subPart {
			subUnique[sc] = struct{}{}
		}

		if len(subUnique) > 1 {
			// Recurse on the sub-partition if any sub-cluster is still oversized
			subPart = SubClusterOversized(subGraph, subPart, localGamma, cfg, seed+1, currentDepth+1)

			// Assign fresh unique community IDs deterministically
			subCommMapping := make(map[int]int)
			for subIdx, sc := range subPart {
				targetComm, exists := subCommMapping[sc]
				if !exists {
					targetComm = nextFreshCommID
					nextFreshCommID++
					subCommMapping[sc] = targetComm
				}
				origIdx := memberToOrig[subIdx]
				finalPartition[origIdx] = targetComm
			}
		}
	}

	return normalizePartition(finalPartition)
}

// extractInducedSubgraph extracts an induced subgraph for a given subset of original graph node indices.
func extractInducedSubgraph(g *Graph, members []int) (*Graph, []int) {
	m := len(members)
	origToSub := make(map[int]int, m)
	memberToOrig := make([]int, m)
	subNodeIDs := make([]string, m)
	subIDToIndex := make(map[string]int, m)

	for subIdx, origIdx := range members {
		origToSub[origIdx] = subIdx
		memberToOrig[subIdx] = origIdx
		id := g.NodeIDs[origIdx]
		subNodeIDs[subIdx] = id
		subIDToIndex[id] = subIdx
	}

	subNodeWeights := make([]float64, m)
	subSelfLoops := make([]float64, m)
	subNeighbors := make([][]WeightedNeighbor, m)
	subDegrees := make([]int, m)
	var subTotalWeight float64

	for subIdx, origIdx := range members {
		subNodeWeights[subIdx] = g.NodeWeights[origIdx]
		subSelfLoops[subIdx] = g.SelfLoops[origIdx]
		subTotalWeight += subSelfLoops[subIdx]

		for _, nb := range g.Neighbors[origIdx] {
			if targetSub, inSub := origToSub[nb.Target]; inSub {
				subNeighbors[subIdx] = append(subNeighbors[subIdx], WeightedNeighbor{
					Target: targetSub,
					Weight: nb.Weight,
				})
				subDegrees[subIdx]++
				if subIdx < targetSub {
					subTotalWeight += nb.Weight
				}
			}
		}
	}

	for i := 0; i < m; i++ {
		sort.Slice(subNeighbors[i], func(a, b int) bool {
			return subNeighbors[i][a].Target < subNeighbors[i][b].Target
		})
	}

	subGraph := &Graph{
		NodeIDs:     subNodeIDs,
		IDToIndex:   subIDToIndex,
		NodeWeights: subNodeWeights,
		SelfLoops:   subSelfLoops,
		Neighbors:   subNeighbors,
		Degrees:     subDegrees,
		TotalWeight: subTotalWeight,
	}

	return subGraph, memberToOrig
}
