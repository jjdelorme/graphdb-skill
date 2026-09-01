package e2e_test

import (
	"sort"
	"testing"
)

// SeamCandidate represents a candidate interface/function seam evaluated for architectural extraction.
type SeamCandidate struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	File                string  `json:"file"`
	InternalFanIn       int     `json:"internal_fan_in"`       // Upstream callers within the candidate boundary
	VolatileFanOut      int     `json:"volatile_fan_out"`      // Downstream dependencies marked volatile/high-churn
	CutEdges            int     `json:"cut_edges"`             // Cross-boundary cut edges severed on extraction
	ActionableSeamScore float64 `json:"actionable_seam_score"` // Calculated ROI score
	Tier                string  `json:"tier"`                  // "Tier 1", "Tier 2", or "Trivial"
	Community           string  `json:"community"`
	Domain              string  `json:"domain"`
}

// ComputeActionableSeamScore computes the Feathers Actionable Seam Score:
// Score = (Internal Fan-In * Volatile Fan-Out) / (Cut-Edges + 1)
func ComputeActionableSeamScore(internalFanIn, volatileFanOut, cutEdges int) float64 {
	if internalFanIn < 0 || volatileFanOut < 0 || cutEdges < 0 {
		return 0.0
	}
	numerator := float64(internalFanIn * volatileFanOut)
	denominator := float64(cutEdges + 1)
	return numerator / denominator
}

// ClassifyAndRankSeams evaluates candidate seams, filters Tier 1 vs Tier 2, and sorts by score descending.
func ClassifyAndRankSeams(candidates []SeamCandidate) (tier1 []SeamCandidate, tier2 []SeamCandidate, other []SeamCandidate) {
	for _, c := range candidates {
		c.ActionableSeamScore = ComputeActionableSeamScore(c.InternalFanIn, c.VolatileFanOut, c.CutEdges)
		if c.CutEdges <= 4 && c.ActionableSeamScore >= 10.0 {
			c.Tier = "Tier 1"
			tier1 = append(tier1, c)
		} else if c.CutEdges > 4 && c.ActionableSeamScore >= 10.0 {
			c.Tier = "Tier 2"
			tier2 = append(tier2, c)
		} else {
			c.Tier = "Trivial"
			other = append(other, c)
		}
	}

	sort.Slice(tier1, func(i, j int) bool {
		return tier1[i].ActionableSeamScore > tier1[j].ActionableSeamScore
	})
	sort.Slice(tier2, func(i, j int) bool {
		return tier2[i].ActionableSeamScore > tier2[j].ActionableSeamScore
	})

	return tier1, tier2, other
}

// TestE2E_Feathers_ActionableSeamScoreCalculation verifies the mathematical correctness of
// the Actionable Seam Score formula across canonical benchmark cases.
func TestE2E_Feathers_ActionableSeamScoreCalculation(t *testing.T) {
	testCases := []struct {
		name           string
		internalFanIn  int
		volatileFanOut int
		cutEdges       int
		expectedScore  float64
		expectedTier1  bool
	}{
		{
			name:           "Candidate A (Ideal High-ROI Tier 1 Seam)",
			internalFanIn:  25,
			volatileFanOut: 20,
			cutEdges:       2,
			expectedScore:  500.0 / 3.0, // 166.6667
			expectedTier1:  true,
		},
		{
			name:           "Candidate B (Tightly Bound Boundary Seam - Exactly 4 Cut-Edges)",
			internalFanIn:  12,
			volatileFanOut: 8,
			cutEdges:       4,
			expectedScore:  96.0 / 5.0, // 19.20
			expectedTier1:  true,
		},
		{
			name:           "Candidate C (Diffuse Monolith Leak - 18 Cut-Edges)",
			internalFanIn:  30,
			volatileFanOut: 30,
			cutEdges:       18,
			expectedScore:  900.0 / 19.0, // 47.3684
			expectedTier1:  false,        // Cut-Edges > 4 forces Tier 2
		},
		{
			name:           "Candidate D (Trivial Low-Fanout Seam)",
			internalFanIn:  2,
			volatileFanOut: 1,
			cutEdges:       1,
			expectedScore:  2.0 / 2.0, // 1.0
			expectedTier1:  false,      // Score < 10.0
		},
		{
			name:           "Candidate E (Zero Volatile Dependencies)",
			internalFanIn:  50,
			volatileFanOut: 0,
			cutEdges:       1,
			expectedScore:  0.0,
			expectedTier1:  false,
		},
		{
			name:           "Candidate F (Single Pinch Point with 0 Cut-Edges)",
			internalFanIn:  10,
			volatileFanOut: 5,
			cutEdges:       0,
			expectedScore:  50.0 / 1.0, // 50.0
			expectedTier1:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := ComputeActionableSeamScore(tc.internalFanIn, tc.volatileFanOut, tc.cutEdges)
			diff := score - tc.expectedScore
			if diff < -1e-4 || diff > 1e-4 {
				t.Errorf("Score mismatch for %s: expected %.4f, got %.4f", tc.name, tc.expectedScore, score)
			}

			isTier1 := tc.cutEdges <= 4 && score >= 10.0
			if isTier1 != tc.expectedTier1 {
				t.Errorf("Tier 1 classification mismatch for %s: expected %v, got %v",
					tc.name, tc.expectedTier1, isTier1)
			}
		})
	}
}

// TestE2E_Feathers_ActionableSeamRanker asserts that the ranker correctly surfaces
// Tier 1 seams (<= 4 cut-edges, score >= 10.0) at the top of recommendations while classifying
// diffuse monolith leaks (> 4 cut-edges) into Tier 2 Background Monolith Debt.
func TestE2E_Feathers_ActionableSeamRanker(t *testing.T) {
	candidates := []SeamCandidate{
		{
			ID:             "cand_a",
			Name:           "PaymentGatewayClient::ProcessPayment",
			File:           "src/payment/gateway.go",
			InternalFanIn:  25,
			VolatileFanOut: 20,
			CutEdges:       2,
			Community:      "comm-1",
			Domain:         "Billing",
		},
		{
			ID:             "cand_b",
			Name:           "AuthSessionManager::ValidateSession",
			File:           "src/auth/session.go",
			InternalFanIn:  12,
			VolatileFanOut: 8,
			CutEdges:       4,
			Community:      "comm-2",
			Domain:         "Identity",
		},
		{
			ID:             "cand_c",
			Name:           "MonolithOrderGodClass::ExecuteOrderWorkflow",
			File:           "src/legacy/order_monolith.go",
			InternalFanIn:  30,
			VolatileFanOut: 30,
			CutEdges:       18,
			Community:      "comm-3",
			Domain:         "Orders",
		},
		{
			ID:             "cand_d",
			Name:           "StringUtil::FormatCurrency",
			File:           "src/util/strings.go",
			InternalFanIn:  2,
			VolatileFanOut: 1,
			CutEdges:       1,
			Community:      "comm-4",
			Domain:         "Core",
		},
		{
			ID:             "cand_e",
			Name:           "StaticLookup::GetCountryCode",
			File:           "src/data/country.go",
			InternalFanIn:  50,
			VolatileFanOut: 0,
			CutEdges:       1,
			Community:      "comm-5",
			Domain:         "Core",
		},
		{
			ID:             "cand_f",
			Name:           "NotificationQueue::DispatchEmailEvent",
			File:           "src/notify/email.go",
			InternalFanIn:  15,
			VolatileFanOut: 10,
			CutEdges:       3,
			Community:      "comm-6",
			Domain:         "Notifications",
		},
	}

	tier1, tier2, other := ClassifyAndRankSeams(candidates)

	// 1. Verify Tier 1 count and ranking
	// Expect Candidate A (166.67), Candidate F (37.50), Candidate B (19.20) in Tier 1
	if len(tier1) != 3 {
		t.Fatalf("Expected 3 Tier 1 seams, got %d", len(tier1))
	}

	if tier1[0].ID != "cand_a" {
		t.Errorf("Rank #1 should be cand_a, got %s (score %.2f)", tier1[0].ID, tier1[0].ActionableSeamScore)
	}
	if tier1[0].ActionableSeamScore < 166.0 {
		t.Errorf("Expected cand_a score ~166.67, got %.2f", tier1[0].ActionableSeamScore)
	}

	if tier1[1].ID != "cand_f" {
		t.Errorf("Rank #2 should be cand_f, got %s (score %.2f)", tier1[1].ID, tier1[1].ActionableSeamScore)
	}

	if tier1[2].ID != "cand_b" {
		t.Errorf("Rank #3 should be cand_b, got %s (score %.2f)", tier1[2].ID, tier1[2].ActionableSeamScore)
	}

	// All Tier 1 seams must strictly satisfy cut_edges <= 4 and score >= 10.0
	for _, s := range tier1 {
		if s.CutEdges > 4 {
			t.Errorf("Tier 1 seam %s has cut-edges %d > 4", s.ID, s.CutEdges)
		}
		if s.ActionableSeamScore < 10.0 {
			t.Errorf("Tier 1 seam %s has score %.2f < 10.0", s.ID, s.ActionableSeamScore)
		}
	}

	// 2. Verify Tier 2 monolith debt classification
	// Candidate C must be in Tier 2 because Cut-Edges = 18 > 4
	if len(tier2) != 1 {
		t.Fatalf("Expected 1 Tier 2 debt item, got %d", len(tier2))
	}
	if tier2[0].ID != "cand_c" {
		t.Errorf("Expected cand_c in Tier 2 debt, got %s", tier2[0].ID)
	}
	if tier2[0].CutEdges <= 4 {
		t.Errorf("Expected cand_c cut-edges > 4, got %d", tier2[0].CutEdges)
	}

	// 3. Verify Other (Trivial) items
	if len(other) != 2 {
		t.Fatalf("Expected 2 trivial items in other, got %d", len(other))
	}
}

// TestE2E_Feathers_BoundarySeamThresholds verifies behavior on exact cutoff boundaries (Cut-Edges=4 vs 5, Score=9.99 vs 10.00).
func TestE2E_Feathers_BoundarySeamThresholds(t *testing.T) {
	testCandidates := []SeamCandidate{
		{
			ID:             "boundary_exact_4_cut",
			InternalFanIn:  5,
			VolatileFanOut: 10,
			CutEdges:       4, // Exactly 4 cut edges -> Score = 50 / 5 = 10.00 -> Tier 1
		},
		{
			ID:             "boundary_5_cut",
			InternalFanIn:  6,
			VolatileFanOut: 10,
			CutEdges:       5, // 5 cut edges -> Score = 60 / 6 = 10.00 -> Tier 2 (CutEdges > 4)
		},
		{
			ID:             "boundary_sub_10_score",
			InternalFanIn:  9,
			VolatileFanOut: 3,
			CutEdges:       2, // Score = 27 / 3 = 9.00 -> Trivial (Score < 10.00)
		},
	}

	tier1, tier2, other := ClassifyAndRankSeams(testCandidates)

	if len(tier1) != 1 || tier1[0].ID != "boundary_exact_4_cut" {
		t.Errorf("Expected boundary_exact_4_cut in Tier 1, got tier1=%+v", tier1)
	}
	if len(tier2) != 1 || tier2[0].ID != "boundary_5_cut" {
		t.Errorf("Expected boundary_5_cut in Tier 2, got tier2=%+v", tier2)
	}
	if len(other) != 1 || other[0].ID != "boundary_sub_10_score" {
		t.Errorf("Expected boundary_sub_10_score in other, got other=%+v", other)
	}
}
