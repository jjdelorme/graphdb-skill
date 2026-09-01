package analysis

import (
	"testing"
)

func TestComputeActionableSeamScore(t *testing.T) {
	tests := []struct {
		name           string
		internalFanIn  int
		volatileFanOut int
		cutEdges       int
		expectedScore  float64
		isTier1        bool
	}{
		{
			name:           "Standard high-ROI seam",
			internalFanIn:  25,
			volatileFanOut: 20,
			cutEdges:       2,
			expectedScore:  500.0 / 3.0,
			isTier1:        true,
		},
		{
			name:           "Exact boundary 4 cut edges",
			internalFanIn:  12,
			volatileFanOut: 8,
			cutEdges:       4,
			expectedScore:  96.0 / 5.0,
			isTier1:        true,
		},
		{
			name:           "Diffuse leak > 4 cut edges",
			internalFanIn:  30,
			volatileFanOut: 30,
			cutEdges:       18,
			expectedScore:  900.0 / 19.0,
			isTier1:        false,
		},
		{
			name:           "Trivial score < 10",
			internalFanIn:  2,
			volatileFanOut: 1,
			cutEdges:       1,
			expectedScore:  1.0,
			isTier1:        false,
		},
		{
			name:           "Zero volatile fanout",
			internalFanIn:  50,
			volatileFanOut: 0,
			cutEdges:       1,
			expectedScore:  0.0,
			isTier1:        false,
		},
		{
			name:           "Negative inputs return 0",
			internalFanIn:  -5,
			volatileFanOut: 10,
			cutEdges:       2,
			expectedScore:  0.0,
			isTier1:        false,
		},
		{
			name:           "Zero cut edges",
			internalFanIn:  10,
			volatileFanOut: 5,
			cutEdges:       0,
			expectedScore:  50.0,
			isTier1:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := ComputeActionableSeamScore(tc.internalFanIn, tc.volatileFanOut, tc.cutEdges)
			diff := score - tc.expectedScore
			if diff < -1e-4 || diff > 1e-4 {
				t.Errorf("expected score %.4f, got %.4f", tc.expectedScore, score)
			}
			tier1 := IsTier1Seam(score, tc.cutEdges)
			if tier1 != tc.isTier1 {
				t.Errorf("expected isTier1 %v, got %v", tc.isTier1, tier1)
			}
		})
	}
}

func TestClassifyAndRankSeams(t *testing.T) {
	candidates := []SeamCandidate{
		{
			ID:             "cand_1",
			InternalFanIn:  25,
			VolatileFanOut: 20,
			CutEdges:       2,
		},
		{
			ID:             "cand_2",
			InternalFanIn:  12,
			VolatileFanOut: 8,
			CutEdges:       4,
		},
		{
			ID:             "cand_3",
			InternalFanIn:  30,
			VolatileFanOut: 30,
			CutEdges:       18,
		},
		{
			ID:             "cand_4",
			InternalFanIn:  2,
			VolatileFanOut: 1,
			CutEdges:       1,
		},
	}

	tier1, tier2, other := ClassifyAndRankSeams(candidates)

	if len(tier1) != 2 {
		t.Fatalf("expected 2 Tier 1 candidates, got %d", len(tier1))
	}
	if tier1[0].ID != "cand_1" {
		t.Errorf("expected cand_1 as top tier 1, got %s", tier1[0].ID)
	}
	if tier1[1].ID != "cand_2" {
		t.Errorf("expected cand_2 as second tier 1, got %s", tier1[1].ID)
	}

	if len(tier2) != 1 || tier2[0].ID != "cand_3" {
		t.Fatalf("expected cand_3 in Tier 2, got %v", tier2)
	}

	if len(other) != 1 || other[0].ID != "cand_4" {
		t.Fatalf("expected cand_4 in other, got %v", other)
	}
}
