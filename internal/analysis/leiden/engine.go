package leiden

import (
	"fmt"
	"math/rand"
	"sort"
)

type cpmLeidenEngine struct {
	config     Config
	edgeMatrix EdgeWeightMatrix
}

// NewEngine creates an Engine instance with custom configuration and base edge matrix.
func NewEngine(cfg Config, matrix EdgeWeightMatrix) Engine {
	return &cpmLeidenEngine{
		config:     cfg,
		edgeMatrix: matrix,
	}
}

// NewDefaultEngine creates an Engine with default configuration and edge matrix.
func NewDefaultEngine() Engine {
	return &cpmLeidenEngine{
		config:     DefaultConfig(),
		edgeMatrix: DefaultEdgeWeightMatrix(),
	}
}

// Partition executes the complete CPM Leiden clustering pipeline on the input nodes and edges.
func (e *cpmLeidenEngine) Partition(nodes []string, edges []RawEdge) (*PartitionResult, error) {
	if len(nodes) == 0 && len(edges) == 0 {
		return &PartitionResult{
			Communities:      []*Community{},
			SharedBoundaries: []*SharedBoundaryNode{},
			CrossCuttingHubs: []*QuarantinedHubNode{},
			Gamma:            e.config.Gamma,
			Quality:          0.0,
		}, nil
	}

	// 1. Build base graph with multi-edge consolidation and inverse-degree damping
	baseGraph := BuildGraph(nodes, edges, e.edgeMatrix, e.config.SuppressHubs)
	totalNodes := len(baseGraph.NodeIDs)
	if totalNodes == 0 {
		return &PartitionResult{
			Communities:      []*Community{},
			SharedBoundaries: []*SharedBoundaryNode{},
			CrossCuttingHubs: []*QuarantinedHubNode{},
			Gamma:            e.config.Gamma,
			Quality:          0.0,
		}, nil
	}

	// 2. Quarantine top 1% hubs (deg > mu + 3*sigma)
	hq := IdentifyAndQuarantineHubs(baseGraph, e.config.SuppressHubs)
	activeGraph := hq.ActiveGraph
	activeN := len(activeGraph.NodeIDs)

	seed := e.config.RandomSeed
	if seed == 0 {
		seed = 42
	}

	var usedGamma float64
	var activePartition []int

	// 3. Community detection on active graph
	if activeN == 0 {
		activePartition = nil
		usedGamma = e.config.Gamma
		if usedGamma <= 0 {
			usedGamma = 0.01
		}
	} else if e.config.Gamma > 0 {
		usedGamma = e.config.Gamma
		rng := rand.New(rand.NewSource(seed))
		activePartition = LeidenClustering(activeGraph, usedGamma, e.config.MaxIterations, rng)
	} else {
		usedGamma, activePartition = SearchOptimalGamma(activeGraph, e.config, seed)
	}

	// 4. Recursive hierarchical sub-clustering on oversized communities (> MaxCommunitySize)
	if activeN > 0 && len(activePartition) > 0 {
		activePartition = SubClusterOversized(activeGraph, activePartition, usedGamma, e.config, seed, 0)
	}

	// 5. Group active nodes into communities
	commMembersMap := make(map[int][]string)
	if activeN > 0 && len(activePartition) > 0 {
		for actIdx, c := range activePartition {
			nodeID := activeGraph.NodeIDs[actIdx]
			commMembersMap[c] = append(commMembersMap[c], nodeID)
		}
	}

	// Sort community keys for deterministic output ordering
	var commKeys []int
	for c := range commMembersMap {
		commKeys = append(commKeys, c)
	}
	sort.Ints(commKeys)

	communities := make([]*Community, 0, len(commKeys))
	for rank, c := range commKeys {
		memberIDs := commMembersMap[c]
		// Sort member IDs deterministically
		sort.Strings(memberIDs)

		commID := fmt.Sprintf("comm-%d", rank)
		commName := fmt.Sprintf("Community %d", rank)

		// Calculate internal edge count & density
		memberSet := make(map[string]struct{}, len(memberIDs))
		for _, id := range memberIDs {
			memberSet[id] = struct{}{}
		}

		var internalEdgeCount int64
		var internalWeight float64
		for _, id := range memberIDs {
			origIdx, ok := baseGraph.IDToIndex[id]
			if !ok {
				continue
			}
			internalWeight += baseGraph.SelfLoops[origIdx]
			for _, nb := range baseGraph.Neighbors[origIdx] {
				targetID := baseGraph.NodeIDs[nb.Target]
				if origIdx < nb.Target {
					if _, inComm := memberSet[targetID]; inComm {
						internalEdgeCount++
						internalWeight += nb.Weight
					}
				}
			}
		}

		size := int64(len(memberIDs))
		var density float64
		if size > 1 {
			possiblePairs := float64(size * (size - 1) / 2)
			density = float64(internalEdgeCount) / possiblePairs
		} else if size == 1 {
			density = 1.0
		}

		communities = append(communities, &Community{
			ID:                commID,
			Name:              commName,
			Gamma:             usedGamma,
			Size:              size,
			Density:           density,
			InternalEdgeCount: internalEdgeCount,
			InternalWeight:    internalWeight,
			TotalWeight:       float64(size),
			NodeIDs:           memberIDs,
		})
	}

	// 6. Post-clustering reintegration of quarantined hubs
	hq.ReintegrateHubs(baseGraph, communities)

	// If all nodes were quarantined (rare extreme case), make a fallback community
	if len(communities) == 0 && len(hq.Hubs) > 0 {
		var hubIDs []string
		for _, h := range hq.Hubs {
			hubIDs = append(hubIDs, h.NodeID)
		}
		sort.Strings(hubIDs)
		communities = append(communities, &Community{
			ID:                "comm-0",
			Name:              "Community 0",
			Gamma:             usedGamma,
			Size:              int64(len(hubIDs)),
			Density:           1.0,
			InternalEdgeCount: 0,
			NodeIDs:           hubIDs,
		})
	}

	// 7. Boundary Participation Ratio (BPR) computation
	sharedBoundaries, avgBPRs := AnalyzeBPR(baseGraph, communities)

	for _, comm := range communities {
		if avg, exists := avgBPRs[comm.ID]; exists {
			comm.BPRAvg = avg
		}
	}

	// 8. Calculate total CPM quality on the base graph partition
	origPartition := make([]int, totalNodes)
	for i := 0; i < totalNodes; i++ {
		origPartition[i] = -1
	}
	for commIdx, comm := range communities {
		for _, id := range comm.NodeIDs {
			if idx, ok := baseGraph.IDToIndex[id]; ok {
				origPartition[idx] = commIdx
			}
		}
	}
	// For any unassigned nodes, make them individual singletons
	for i := 0; i < totalNodes; i++ {
		if origPartition[i] == -1 {
			origPartition[i] = len(communities) + i
		}
	}

	quality := CalculateQuality(baseGraph, origPartition, usedGamma)

	return &PartitionResult{
		Communities:      communities,
		SharedBoundaries: sharedBoundaries,
		CrossCuttingHubs: hq.Hubs,
		Gamma:            usedGamma,
		Quality:          quality,
	}, nil
}
