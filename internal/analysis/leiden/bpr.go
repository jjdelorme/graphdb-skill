package leiden

// AnalyzeBPR computes Boundary Participation Ratios for all nodes across discovered communities,
// identifies shared boundary interface components, and computes community average BPRs.
func AnalyzeBPR(
	baseGraph *Graph,
	communities []*Community,
) ([]*SharedBoundaryNode, map[string]float64) {
	n := len(baseGraph.NodeIDs)
	if n == 0 || len(communities) == 0 {
		return nil, make(map[string]float64)
	}

	// Map each base node ID to its community ID
	nodeToComm := make(map[string]string)
	for _, comm := range communities {
		for _, nodeID := range comm.NodeIDs {
			nodeToComm[nodeID] = comm.ID
		}
	}

	var sharedBoundaries []*SharedBoundaryNode
	commBPRSums := make(map[string]float64)
	commNodeCounts := make(map[string]int)

	for _, comm := range communities {
		commNodeCounts[comm.ID] = len(comm.NodeIDs)
	}

	for i := 0; i < n; i++ {
		nodeID := baseGraph.NodeIDs[i]

		var totalIncidentWt float64
		commIncidentWt := make(map[string]float64)

		for _, nb := range baseGraph.Neighbors[i] {
			targetID := baseGraph.NodeIDs[nb.Target]
			totalIncidentWt += nb.Weight

			if targetCommID, exists := nodeToComm[targetID]; exists {
				commIncidentWt[targetCommID] += nb.Weight
			}
		}

		if totalIncidentWt <= 0 {
			continue
		}

		// Compute BPR per community
		bprMap := make(map[string]float64)
		var bprMax float64
		var countOver25 int

		for _, comm := range communities {
			inc := commIncidentWt[comm.ID]
			bpr := inc / totalIncidentWt
			if bpr > 0 {
				bprMap[comm.ID] = bpr
			}
			if bpr > bprMax {
				bprMax = bpr
			}
			if bpr >= 0.25 {
				countOver25++
			}
		}

		// Record own community BPR for average calculation
		if ownCommID, exists := nodeToComm[nodeID]; exists {
			commBPRSums[ownCommID] += bprMap[ownCommID]
		}

		// Shared boundary condition: BPR >= 0.25 across >= 2 communities
		if countOver25 >= 2 {
			// Record links for all communities with BPR >= 0.15
			filteredBPRs := make(map[string]float64)
			for commID, ratio := range bprMap {
				if ratio >= 0.15 {
					filteredBPRs[commID] = ratio
				}
			}

			sharedBoundaries = append(sharedBoundaries, &SharedBoundaryNode{
				NodeID:                 nodeID,
				BPRMax:                 bprMax,
				BoundaryCommunityCount: countOver25,
				CommunityBPRs:          filteredBPRs,
			})
		}
	}

	avgBPRs := make(map[string]float64, len(communities))
	for _, comm := range communities {
		count := commNodeCounts[comm.ID]
		if count > 0 {
			avgBPRs[comm.ID] = commBPRSums[comm.ID] / float64(count)
		}
	}

	return sharedBoundaries, avgBPRs
}
