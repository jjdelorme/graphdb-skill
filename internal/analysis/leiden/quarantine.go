package leiden

import (
	"math"
	"sort"
)

// HubQuarantine holds isolated hub information and the mapping to the active graph.
type HubQuarantine struct {
	Hubs         []*QuarantinedHubNode
	HubIndices   map[int]struct{}
	ActiveGraph  *Graph
	ActiveToOrig []int // Active node index -> Original node index
	OrigToActive []int // Original node index -> Active node index (-1 if quarantined)
	MeanDegree   float64
	StdDegree    float64
}

// IdentifyAndQuarantineHubs identifies outlier nodes with degree > mu + 3*sigma and constructs the active subgraph.
func IdentifyAndQuarantineHubs(baseGraph *Graph, suppressHubs bool) *HubQuarantine {
	n := len(baseGraph.NodeIDs)
	if n == 0 {
		return &HubQuarantine{
			Hubs:         nil,
			HubIndices:   make(map[int]struct{}),
			ActiveGraph:  baseGraph,
			ActiveToOrig: nil,
			OrigToActive: nil,
			MeanDegree:   0,
			StdDegree:    0,
		}
	}

	if !suppressHubs || n < 10 {
		activeToOrig := make([]int, n)
		origToActive := make([]int, n)
		for i := 0; i < n; i++ {
			activeToOrig[i] = i
			origToActive[i] = i
		}
		return &HubQuarantine{
			Hubs:         nil,
			HubIndices:   make(map[int]struct{}),
			ActiveGraph:  baseGraph,
			ActiveToOrig: activeToOrig,
			OrigToActive: origToActive,
			MeanDegree:   0,
			StdDegree:    0,
		}
	}

	// Compute degree mean and standard deviation
	var sumDeg float64
	degrees := make([]int, n)
	copy(degrees, baseGraph.Degrees)

	for _, d := range degrees {
		sumDeg += float64(d)
	}
	mean := sumDeg / float64(n)

	var varianceSum float64
	for _, d := range degrees {
		diff := float64(d) - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(n))

	// Compute 99th percentile
	sortedDegs := make([]int, n)
	copy(sortedDegs, degrees)
	sort.Ints(sortedDegs)
	p99Idx := int(float64(n-1) * 0.99)
	p99 := float64(sortedDegs[p99Idx])

	threshold := mean + 3.0*stdDev
	if p99 > threshold {
		threshold = p99
	}
	// Absolute minimum floor for hub threshold in medium/large graphs
	if n >= 50 && threshold < 25.0 {
		threshold = 25.0
	}

	hubIndices := make(map[int]struct{})
	var quarantined []*QuarantinedHubNode

	for i, d := range degrees {
		degFloat := float64(d)
		if degFloat > threshold && (stdDev == 0 || degFloat > mean+3.0*stdDev) {
			hubIndices[i] = struct{}{}
			var zScore float64
			if stdDev > 0 {
				zScore = (degFloat - mean) / stdDev
			}
			quarantined = append(quarantined, &QuarantinedHubNode{
				NodeID:              baseGraph.NodeIDs[i],
				Degree:              d,
				HubScore:            zScore,
				CommunityAffinities: make(map[string]float64),
			})
		}
	}

	// If no hubs identified or too many (> 10% of graph), don't quarantine
	if len(quarantined) == 0 || len(quarantined) > n/10 {
		activeToOrig := make([]int, n)
		origToActive := make([]int, n)
		for i := 0; i < n; i++ {
			activeToOrig[i] = i
			origToActive[i] = i
		}
		return &HubQuarantine{
			Hubs:         nil,
			HubIndices:   make(map[int]struct{}),
			ActiveGraph:  baseGraph,
			ActiveToOrig: activeToOrig,
			OrigToActive: origToActive,
			MeanDegree:   mean,
			StdDegree:    stdDev,
		}
	}

	// Build active subgraph
	activeToOrig := make([]int, 0, n-len(hubIndices))
	origToActive := make([]int, n)
	for i := 0; i < n; i++ {
		origToActive[i] = -1
	}

	activeNodeIDs := make([]string, 0, n-len(hubIndices))
	activeIDToIndex := make(map[string]int, n-len(hubIndices))

	for i := 0; i < n; i++ {
		if _, isHub := hubIndices[i]; !isHub {
			actIdx := len(activeToOrig)
			activeToOrig = append(activeToOrig, i)
			origToActive[i] = actIdx
			id := baseGraph.NodeIDs[i]
			activeNodeIDs = append(activeNodeIDs, id)
			activeIDToIndex[id] = actIdx
		}
	}

	activeN := len(activeToOrig)
	activeNodeWeights := make([]float64, activeN)
	activeSelfLoops := make([]float64, activeN)
	activeNeighbors := make([][]WeightedNeighbor, activeN)
	activeDegrees := make([]int, activeN)
	var activeTotalWeight float64

	for actIdx, origIdx := range activeToOrig {
		activeNodeWeights[actIdx] = baseGraph.NodeWeights[origIdx]
		activeSelfLoops[actIdx] = baseGraph.SelfLoops[origIdx]
		activeTotalWeight += activeSelfLoops[actIdx]

		for _, nb := range baseGraph.Neighbors[origIdx] {
			if actTarget := origToActive[nb.Target]; actTarget != -1 {
				activeNeighbors[actIdx] = append(activeNeighbors[actIdx], WeightedNeighbor{
					Target: actTarget,
					Weight: nb.Weight,
				})
				activeDegrees[actIdx]++
				if actIdx < actTarget {
					activeTotalWeight += nb.Weight
				}
			}
		}
		sort.Slice(activeNeighbors[actIdx], func(a, b int) bool {
			return activeNeighbors[actIdx][a].Target < activeNeighbors[actIdx][b].Target
		})
	}

	activeGraph := &Graph{
		NodeIDs:     activeNodeIDs,
		IDToIndex:   activeIDToIndex,
		NodeWeights: activeNodeWeights,
		SelfLoops:   activeSelfLoops,
		Neighbors:   activeNeighbors,
		Degrees:     activeDegrees,
		TotalWeight: activeTotalWeight,
	}

	return &HubQuarantine{
		Hubs:         quarantined,
		HubIndices:   hubIndices,
		ActiveGraph:  activeGraph,
		ActiveToOrig: activeToOrig,
		OrigToActive: origToActive,
		MeanDegree:   mean,
		StdDegree:    stdDev,
	}
}

// ReintegrateHubs computes community affinities for quarantined hubs and attaches them where applicable.
func (hq *HubQuarantine) ReintegrateHubs(baseGraph *Graph, communities []*Community) {
	if len(hq.Hubs) == 0 || len(communities) == 0 {
		return
	}

	// Map community ID to node index set (in original graph)
	commNodeSets := make(map[string]map[int]struct{}, len(communities))
	for _, comm := range communities {
		set := make(map[int]struct{}, len(comm.NodeIDs))
		for _, nodeID := range comm.NodeIDs {
			if origIdx, ok := baseGraph.IDToIndex[nodeID]; ok {
				set[origIdx] = struct{}{}
			}
		}
		commNodeSets[comm.ID] = set
	}

	for _, hub := range hq.Hubs {
		hubIdx, ok := baseGraph.IDToIndex[hub.NodeID]
		if !ok {
			continue
		}

		var totalIncidentToActive float64
		commIncident := make(map[string]float64)

		for _, nb := range baseGraph.Neighbors[hubIdx] {
			if _, isHub := hq.HubIndices[nb.Target]; !isHub {
				totalIncidentToActive += nb.Weight
				// Find which community nb.Target belongs to
				for commID, nodeSet := range commNodeSets {
					if _, inComm := nodeSet[nb.Target]; inComm {
						commIncident[commID] += nb.Weight
						break
					}
				}
			}
		}

		hub.CommunityAffinities = make(map[string]float64)
		if totalIncidentToActive <= 0 {
			// No edges to active graph, attach to first community
			if len(communities) > 0 {
				communities[0].NodeIDs = append(communities[0].NodeIDs, hub.NodeID)
				communities[0].Size++
			}
			continue
		}

		var maxAffinity float64
		var bestCommID string
		assigned := false

		for _, comm := range communities {
			inc := commIncident[comm.ID]
			aff := inc / totalIncidentToActive
			if aff >= 0.10 {
				hub.CommunityAffinities[comm.ID] = aff
			}
			if aff > maxAffinity {
				maxAffinity = aff
				bestCommID = comm.ID
			}

			// Dominant host rule: >= 0.70 affinity assigns hub to the community
			if aff >= 0.70 && !assigned {
				comm.NodeIDs = append(comm.NodeIDs, hub.NodeID)
				comm.Size++
				commNodeSets[comm.ID][hubIdx] = struct{}{}
				assigned = true
			}
		}

		// If no community reached 0.10 affinity, link to the single highest affinity
		if len(hub.CommunityAffinities) == 0 && bestCommID != "" {
			hub.CommunityAffinities[bestCommID] = maxAffinity
		}

		// If hub was not assigned by dominant host rule, attach to best community
		if !assigned {
			targetCommID := bestCommID
			if targetCommID == "" && len(communities) > 0 {
				targetCommID = communities[0].ID
			}
			for _, comm := range communities {
				if comm.ID == targetCommID {
					comm.NodeIDs = append(comm.NodeIDs, hub.NodeID)
					comm.Size++
					commNodeSets[comm.ID][hubIdx] = struct{}{}
					break
				}
			}
		}
	}

	for _, comm := range communities {
		sort.Strings(comm.NodeIDs)
	}
}
