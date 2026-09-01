package query

import (
	"testing"
)

func TestDualLensStructs(t *testing.T) {
	seam := &DualLensSeamResult{
		ID:             "fn1",
		Seam:           "ProcessPayment",
		File:           "payment/gateway.go",
		InternalFanIn:  25,
		VolatileFanOut: 20,
		CutEdges:       2,
		Score:          166.6667,
		Community:      "comm-1",
		Domain:         "Billing",
	}

	if seam.Score < 10.0 || seam.CutEdges > 4 {
		t.Errorf("Seam should satisfy Tier 1 criteria: %+v", seam)
	}

	div := &DomainDivergenceResult{
		DomainID:        "domain_billing",
		DomainName:      "Billing",
		TotalFunctions:  100,
		DivergenceScore: 0.25,
		Distribution: []CommunityDistributionItem{
			{
				CommunityID:   "comm-1",
				CommunityName: "Community 1",
				FunctionCount: 75,
				Ratio:         0.75,
			},
			{
				CommunityID:   "comm-2",
				CommunityName: "Community 2",
				FunctionCount: 25,
				Ratio:         0.25,
			},
		},
	}

	if div.DivergenceScore != 0.25 {
		t.Errorf("Expected divergence score 0.25, got %f", div.DivergenceScore)
	}
	if len(div.Distribution) != 2 {
		t.Errorf("Expected 2 distribution items, got %d", len(div.Distribution))
	}

	comm := &StructuralCommunityResult{
		ID:                   "comm-1",
		Name:                 "Community 1",
		Gamma:                0.05,
		Size:                 75,
		Density:              0.3,
		InternalEdgeCount:    250,
		BPRAvg:               0.06,
		SharedBoundaryCount:  3,
		CrossCuttingHubCount: 1,
		DominantDomains:      []string{"Billing"},
	}

	if comm.Size != 75 || comm.InternalEdgeCount != 250 {
		t.Errorf("StructuralCommunityResult mismatch: %+v", comm)
	}
}
