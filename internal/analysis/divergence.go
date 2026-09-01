package analysis

import (
	"math"
)

// CalculateDomainDivergence computes the domain divergence score:
// Divergence(D) = 1.0 - max_C P(C | D)
// where P(C | D) is the proportion of domain D's functions residing in community C.
// A score of 0.0 means 100% encapsulation in a single community.
// Higher scores indicate increasing structural fragmentation across multiple communities.
func CalculateDomainDivergence(ratios []float64) float64 {
	if len(ratios) == 0 {
		return 0.0
	}
	maxRatio := 0.0
	for _, r := range ratios {
		if r > maxRatio {
			maxRatio = r
		}
	}
	if maxRatio > 1.0 {
		maxRatio = 1.0
	}
	return 1.0 - maxRatio
}

// CalculateDomainEntropy computes Shannon entropy of domain distribution across communities:
// H(D) = - sum_C P(C | D) * ln(P(C | D))
func CalculateDomainEntropy(ratios []float64) float64 {
	if len(ratios) == 0 {
		return 0.0
	}
	entropy := 0.0
	for _, r := range ratios {
		if r > 0.0 {
			entropy -= r * math.Log(r)
		}
	}
	return entropy
}

// CalculateCommunityPurity computes the proportion of a community's members belonging to a specific domain:
// P(D | C) = |F_C \cap F_D| / |F_C|
func CalculateCommunityPurity(domainFuncCount, totalCommunityFuncCount int) float64 {
	if totalCommunityFuncCount <= 0 || domainFuncCount <= 0 {
		return 0.0
	}
	purity := float64(domainFuncCount) / float64(totalCommunityFuncCount)
	if purity > 1.0 {
		purity = 1.0
	}
	return purity
}

// CalculateCommunityEntropy computes Shannon entropy of domain mixing within a community:
// H(C) = - sum_D P(D | C) * ln(P(D | C))
func CalculateCommunityEntropy(domainCounts []int, totalCommunityFuncCount int) float64 {
	if totalCommunityFuncCount <= 0 || len(domainCounts) == 0 {
		return 0.0
	}
	entropy := 0.0
	total := float64(totalCommunityFuncCount)
	for _, cnt := range domainCounts {
		if cnt > 0 {
			p := float64(cnt) / total
			entropy -= p * math.Log(p)
		}
	}
	return entropy
}
