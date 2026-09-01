package analysis

import (
	"math"
	"testing"
)

func TestCalculateDomainDivergence(t *testing.T) {
	tests := []struct {
		name     string
		ratios   []float64
		expected float64
	}{
		{
			name:     "Empty ratios",
			ratios:   nil,
			expected: 0.0,
		},
		{
			name:     "100% encapsulation in single community",
			ratios:   []float64{1.0},
			expected: 0.0,
		},
		{
			name:     "Split 50-50 across 2 communities",
			ratios:   []float64{0.5, 0.5},
			expected: 0.5,
		},
		{
			name:     "Split 70-20-10 across 3 communities",
			ratios:   []float64{0.7, 0.2, 0.1},
			expected: 0.3,
		},
		{
			name:     "Diffuse split across 10 communities (10% each)",
			ratios:   []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
			expected: 0.9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateDomainDivergence(tc.ratios)
			if math.Abs(got-tc.expected) > 1e-6 {
				t.Errorf("expected %.4f, got %.4f", tc.expected, got)
			}
		})
	}
}

func TestCalculateDomainEntropy(t *testing.T) {
	// 100% single community -> entropy 0
	e0 := CalculateDomainEntropy([]float64{1.0})
	if e0 != 0.0 {
		t.Errorf("expected 0.0 entropy, got %f", e0)
	}

	// 50-50 split -> -2 * (0.5 * ln(0.5)) = ln(2) = 0.6931...
	e1 := CalculateDomainEntropy([]float64{0.5, 0.5})
	expectedE1 := math.Ln2
	if math.Abs(e1-expectedE1) > 1e-4 {
		t.Errorf("expected %f, got %f", expectedE1, e1)
	}
}

func TestCalculateCommunityPurity(t *testing.T) {
	p := CalculateCommunityPurity(80, 100)
	if math.Abs(p-0.8) > 1e-6 {
		t.Errorf("expected purity 0.8, got %f", p)
	}

	p0 := CalculateCommunityPurity(0, 100)
	if p0 != 0.0 {
		t.Errorf("expected purity 0.0, got %f", p0)
	}
}

func TestCalculateCommunityEntropy(t *testing.T) {
	counts := []int{50, 50}
	ent := CalculateCommunityEntropy(counts, 100)
	if math.Abs(ent-math.Ln2) > 1e-4 {
		t.Errorf("expected %f, got %f", math.Ln2, ent)
	}
}
