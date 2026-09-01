package leiden

import (
	"math"
	"math/rand"
	"sort"
)

const floatTolerance = 1e-9

// LeidenClustering executes the multi-level Leiden community detection algorithm on graph g.
func LeidenClustering(g *Graph, gamma float64, maxIterations int, rng *rand.Rand) []int {
	n := len(g.NodeIDs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []int{0}
	}
	if maxIterations <= 0 {
		maxIterations = 50
	}

	currentGraph := g
	// Initial partition: singletons
	currentPartition := make([]int, n)
	for i := 0; i < n; i++ {
		currentPartition[i] = i
	}

	var hierarchy [][]int
	var finalTopPartition []int

	for iter := 0; iter < maxIterations; iter++ {
		if len(currentGraph.NodeIDs) <= 1 {
			break
		}

		// Phase 1: Fast local move optimization on currentGraph starting from currentPartition
		optPartition := localMoveOptimization(currentGraph, currentPartition, gamma, rng)

		// Phase 2: Refinement of optPartition on currentGraph
		refinedPartition, numRefined := refinePartition(currentGraph, optPartition, gamma, rng)

		hierarchy = append(hierarchy, refinedPartition)

		// Construct metaPartition (mapping refined community r -> parent community in optPartition)
		metaPartition := make([]int, numRefined)
		for i, r := range refinedPartition {
			metaPartition[r] = optPartition[i]
		}
		metaPartition = normalizePartition(metaPartition)

		// Termination conditions:
		// 1. Every node remained a singleton in refinement (no aggregation possible / cannot coarsen further)
		// 2. Meta-graph is a single node
		if numRefined == len(currentGraph.NodeIDs) || len(currentGraph.NodeIDs) <= 1 {
			finalTopPartition = metaPartition
			break
		}

		// Phase 3: Meta-Graph Aggregation
		metaGraph := aggregateMetaGraph(currentGraph, refinedPartition, numRefined)

		currentGraph = metaGraph
		currentPartition = metaPartition
		finalTopPartition = metaPartition
	}

	if finalTopPartition == nil {
		finalTopPartition = currentPartition
	}

	// Flatten hierarchy back to base graph nodes
	finalPartition := flattenHierarchy(hierarchy, finalTopPartition)
	return normalizePartition(finalPartition)
}

// localMoveOptimization implements Phase 1: Fast local node move using a deterministic queue.
func localMoveOptimization(g *Graph, initialPartition []int, gamma float64, rng *rand.Rand) []int {
	n := len(g.NodeIDs)
	partition := make([]int, n)
	copy(partition, initialPartition)

	// Compute community node weights
	commWeights := make(map[int]float64)
	for i := 0; i < n; i++ {
		commWeights[partition[i]] += g.NodeWeights[i]
	}

	// Initialize queue with all nodes
	queue := make([]int, n)
	for i := 0; i < n; i++ {
		queue[i] = i
	}
	if rng != nil {
		rng.Shuffle(n, func(i, j int) {
			queue[i], queue[j] = queue[j], queue[i]
		})
	}

	inQueue := make([]bool, n)
	for i := 0; i < n; i++ {
		inQueue[i] = true
	}

	freshIDCounter := n * 10
	qHead := 0
	for qHead < len(queue) {
		v := queue[qHead]
		qHead++
		inQueue[v] = false

		srcComm := partition[v]
		wV := g.NodeWeights[v]
		wSrc := commWeights[srcComm]

		// Calculate edge weights to adjacent communities
		commEdges := make(map[int]float64)
		for _, nb := range g.Neighbors[v] {
			commEdges[partition[nb.Target]] += nb.Weight
		}

		eSrc := commEdges[srcComm]

		// Find best target community
		bestGain := 0.0
		bestComm := srcComm

		type candidate struct {
			commID int
			gain   float64
		}
		var candidates []candidate

		// Evaluate moving to an empty community (singleton)
		if wSrc > wV {
			gainEmpty := DeltaH(v, srcComm, -1, eSrc, 0.0, wV, wSrc, 0.0, gamma)
			if gainEmpty > floatTolerance {
				candidates = append(candidates, candidate{commID: -1, gain: gainEmpty})
			}
		}

		// Iterate over sorted adjacent community keys for determinism
		edgeComms := make([]int, 0, len(commEdges))
		for dstComm := range commEdges {
			edgeComms = append(edgeComms, dstComm)
		}
		sort.Ints(edgeComms)

		for _, dstComm := range edgeComms {
			if dstComm == srcComm {
				continue
			}
			eDst := commEdges[dstComm]
			wDst := commWeights[dstComm]
			gain := DeltaH(v, srcComm, dstComm, eSrc, eDst, wV, wSrc, wDst, gamma)
			if gain > floatTolerance {
				candidates = append(candidates, candidate{commID: dstComm, gain: gain})
			}
		}

		// Pick candidate with max gain, breaking ties deterministically with lower ID
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool {
				if math.Abs(candidates[i].gain-candidates[j].gain) > floatTolerance {
					return candidates[i].gain > candidates[j].gain
				}
				return candidates[i].commID < candidates[j].commID
			})

			bestGain = candidates[0].gain
			bestComm = candidates[0].commID
		}

		if bestGain > floatTolerance && bestComm != srcComm {
			targetComm := bestComm
			if targetComm == -1 {
				freshIDCounter++
				targetComm = freshIDCounter
			}

			// Perform move
			commWeights[srcComm] -= wV
			if commWeights[srcComm] <= 0 {
				delete(commWeights, srcComm)
			}
			commWeights[targetComm] += wV
			partition[v] = targetComm

			// Add unqueued neighbors of v to queue
			for _, nb := range g.Neighbors[v] {
				u := nb.Target
				if partition[u] != targetComm && !inQueue[u] {
					inQueue[u] = true
					queue = append(queue, u)
				}
			}
		}
	}

	return normalizePartition(partition)
}

// refinePartition implements Phase 2: Refinement of communities into well-connected sub-clusters.
func refinePartition(g *Graph, optPartition []int, gamma float64, rng *rand.Rand) ([]int, int) {
	n := len(g.NodeIDs)

	// Group nodes by community in optPartition
	commNodes := make(map[int][]int)
	for i := 0; i < n; i++ {
		c := optPartition[i]
		commNodes[c] = append(commNodes[c], i)
	}

	// Sort community keys for deterministic iteration order
	commKeys := make([]int, 0, len(commNodes))
	for c := range commNodes {
		commKeys = append(commKeys, c)
	}
	sort.Ints(commKeys)

	// Initial refined partition: singletons
	refinedComm := make([]int, n)
	refinedWeight := make([]float64, n)
	for i := 0; i < n; i++ {
		refinedComm[i] = i
		refinedWeight[i] = g.NodeWeights[i]
	}

	// Refinement process per community C in optPartition
	for _, cKey := range commKeys {
		nodes := commNodes[cKey]
		if len(nodes) <= 1 {
			continue
		}

		// Calculate total community weight w(C)
		var wC float64
		nodeSet := make(map[int]struct{}, len(nodes))
		for _, u := range nodes {
			wC += g.NodeWeights[u]
			nodeSet[u] = struct{}{}
		}

		// Identify well-connected nodes within C:
		// E(v, C \ {v}) >= gamma * w(v) * (w(C) - w(v))
		var wellConnected []int
		for _, v := range nodes {
			var eInC float64
			for _, nb := range g.Neighbors[v] {
				if _, inC := nodeSet[nb.Target]; inC {
					eInC += nb.Weight
				}
			}
			wV := g.NodeWeights[v]
			threshold := gamma * wV * (wC - wV)
			if eInC >= threshold-floatTolerance {
				wellConnected = append(wellConnected, v)
			}
		}

		if len(wellConnected) == 0 {
			continue
		}

		// Deterministic permutation of well-connected nodes
		if rng != nil {
			rng.Shuffle(len(wellConnected), func(i, j int) {
				wellConnected[i], wellConnected[j] = wellConnected[j], wellConnected[i]
			})
		}

		// Refined community node membership map inside C
		refinedSubNodes := make(map[int][]int)
		for _, u := range nodes {
			refinedSubNodes[u] = []int{u}
		}

		for _, v := range wellConnected {
			// Only consider v if v is still a singleton in refined partition
			if len(refinedSubNodes[refinedComm[v]]) != 1 {
				continue
			}

			wV := g.NodeWeights[v]

			type candidateRefined struct {
				commID int
				gain   float64
			}
			var candidates []candidateRefined

			// Collect neighbor edges to refined sub-clusters inside C
			refinedEdges := make(map[int]float64)
			for _, nb := range g.Neighbors[v] {
				if _, inC := nodeSet[nb.Target]; inC {
					rComm := refinedComm[nb.Target]
					refinedEdges[rComm] += nb.Weight
				}
			}

			rKeys := make([]int, 0, len(refinedEdges))
			for rComm := range refinedEdges {
				rKeys = append(rKeys, rComm)
			}
			sort.Ints(rKeys)

			for _, rComm := range rKeys {
				if rComm == refinedComm[v] {
					continue
				}

				eVR := refinedEdges[rComm]
				wR := refinedWeight[rComm]

				// Check if refined sub-cluster C_r is well-connected within C:
				// E(C_r, C \ C_r) >= gamma * w(C_r) * (w(C) - w(C_r))
				var eRToRestOfC float64
				rMembers := refinedSubNodes[rComm]
				rMemberSet := make(map[int]struct{}, len(rMembers))
				for _, m := range rMembers {
					rMemberSet[m] = struct{}{}
				}

				for _, m := range rMembers {
					for _, nb := range g.Neighbors[m] {
						if _, inC := nodeSet[nb.Target]; inC {
							if _, inR := rMemberSet[nb.Target]; !inR {
								eRToRestOfC += nb.Weight
							}
						}
					}
				}

				rThreshold := gamma * wR * (wC - wR)
				if eRToRestOfC >= rThreshold-floatTolerance {
					// Quality gain of adding v to C_r from empty:
					// DeltaH(v: empty -> C_r) = E(v, C_r) - gamma * w(v) * w(C_r)
					gain := eVR - (gamma * wV * wR)
					if gain > floatTolerance {
						candidates = append(candidates, candidateRefined{commID: rComm, gain: gain})
					}
				}
			}

			if len(candidates) > 0 {
				sort.Slice(candidates, func(i, j int) bool {
					if math.Abs(candidates[i].gain-candidates[j].gain) > floatTolerance {
						return candidates[i].gain > candidates[j].gain
					}
					return candidates[i].commID < candidates[j].commID
				})

				bestTarget := candidates[0].commID
				oldComm := refinedComm[v]

				// Merge v into bestTarget
				refinedComm[v] = bestTarget
				refinedWeight[bestTarget] += wV
				refinedSubNodes[bestTarget] = append(refinedSubNodes[bestTarget], v)

				// Clear old singleton sub-cluster
				refinedSubNodes[oldComm] = nil
			}
		}
	}

	// Renumber refined communities to contiguous 0..numRefined-1
	normRefined := normalizePartition(refinedComm)
	numRefined := 0
	for _, c := range normRefined {
		if c+1 > numRefined {
			numRefined = c + 1
		}
	}

	return normRefined, numRefined
}

// aggregateMetaGraph implements Phase 3: Construction of aggregated meta-graph from refined communities.
func aggregateMetaGraph(
	g *Graph,
	refinedPartition []int,
	numRefined int,
) *Graph {
	n := len(g.NodeIDs)

	metaNodeWeights := make([]float64, numRefined)
	metaSelfLoops := make([]float64, numRefined)
	metaNodeIDs := make([]string, numRefined)
	metaIDToIndex := make(map[string]int, numRefined)

	for i := 0; i < n; i++ {
		r := refinedPartition[i]
		metaNodeWeights[r] += g.NodeWeights[i]
		metaSelfLoops[r] += g.SelfLoops[i]
	}

	for r := 0; r < numRefined; r++ {
		id := string(rune('M')) + string(rune('0'+r))
		metaNodeIDs[r] = id
		metaIDToIndex[id] = r
	}

	// Consolidate inter-meta-node edge weights
	edgeMap := make(map[uint64]float64)
	for u := 0; u < n; u++ {
		rU := refinedPartition[u]
		for _, nb := range g.Neighbors[u] {
			v := nb.Target
			if u < v {
				rV := refinedPartition[v]
				if rU == rV {
					metaSelfLoops[rU] += nb.Weight
				} else {
					key := pairKey(rU, rV)
					edgeMap[key] += nb.Weight
				}
			}
		}
	}

	keys := make([]uint64, 0, len(edgeMap))
	for k := range edgeMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	metaNeighbors := make([][]WeightedNeighbor, numRefined)
	metaDegrees := make([]int, numRefined)
	var metaTotalWeight float64

	for _, key := range keys {
		wt := edgeMap[key]
		u := int(key >> 32)
		v := int(key & 0xFFFFFFFF)

		metaNeighbors[u] = append(metaNeighbors[u], WeightedNeighbor{Target: v, Weight: wt})
		metaNeighbors[v] = append(metaNeighbors[v], WeightedNeighbor{Target: u, Weight: wt})
		metaDegrees[u]++
		metaDegrees[v]++
		metaTotalWeight += wt
	}

	for r := 0; r < numRefined; r++ {
		metaTotalWeight += metaSelfLoops[r]
		sort.Slice(metaNeighbors[r], func(a, b int) bool {
			return metaNeighbors[r][a].Target < metaNeighbors[r][b].Target
		})
	}

	return &Graph{
		NodeIDs:     metaNodeIDs,
		IDToIndex:   metaIDToIndex,
		NodeWeights: metaNodeWeights,
		SelfLoops:   metaSelfLoops,
		Neighbors:   metaNeighbors,
		Degrees:     metaDegrees,
		TotalWeight: metaTotalWeight,
	}
}

// flattenHierarchy traces community assignments through all refinement levels down to base nodes.
func flattenHierarchy(hierarchy [][]int, topPartition []int) []int {
	if len(hierarchy) == 0 {
		return topPartition
	}

	currentMapping := topPartition
	for lvl := len(hierarchy) - 1; lvl >= 0; lvl-- {
		refined := hierarchy[lvl]
		newLvlMapping := make([]int, len(refined))
		for i, r := range refined {
			if r < len(currentMapping) {
				newLvlMapping[i] = currentMapping[r]
			} else {
				newLvlMapping[i] = r
			}
		}
		currentMapping = newLvlMapping
	}

	return currentMapping
}

// normalizePartition relabels partition community IDs to contiguous integers 0..K-1.
func normalizePartition(partition []int) []int {
	labelMap := make(map[int]int)
	normalized := make([]int, len(partition))

	for i, c := range partition {
		if newID, exists := labelMap[c]; exists {
			normalized[i] = newID
		} else {
			newID = len(labelMap)
			labelMap[c] = newID
			normalized[i] = newID
		}
	}

	return normalized
}
