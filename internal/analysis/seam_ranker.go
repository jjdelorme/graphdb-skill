package analysis

import (
	"sort"
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

// IsTier1Seam returns true if the candidate satisfies Tier 1 criteria:
// Cut-Edges <= 4 AND ActionableSeamScore >= 10.0.
func IsTier1Seam(score float64, cutEdges int) bool {
	return cutEdges <= 4 && score >= 10.0
}

// IsTier2Debt returns true if the candidate represents background monolith debt (Cut-Edges > 4 and Score >= 10.0).
func IsTier2Debt(score float64, cutEdges int) bool {
	return cutEdges > 4 && score >= 10.0
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
