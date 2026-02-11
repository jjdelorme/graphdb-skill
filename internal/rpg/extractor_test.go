package rpg

import (
	"testing"
)

func TestMockFeatureExtractor_Extract(t *testing.T) {
	extractor := &MockFeatureExtractor{}

	descriptors, err := extractor.Extract("func login() { validate(); hash(); }", "login")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(descriptors) != 2 {
		t.Fatalf("Expected 2 descriptors, got %d", len(descriptors))
	}
	if descriptors[0] != "process data" {
		t.Errorf("Expected 'process data', got '%s'", descriptors[0])
	}
	if descriptors[1] != "validate input" {
		t.Errorf("Expected 'validate input', got '%s'", descriptors[1])
	}
}

func TestMockFeatureExtractor_EmptyCode(t *testing.T) {
	extractor := &MockFeatureExtractor{}

	descriptors, err := extractor.Extract("", "empty")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// MockFeatureExtractor returns fixed values regardless of input
	if len(descriptors) != 2 {
		t.Fatalf("Expected 2 descriptors from mock, got %d", len(descriptors))
	}
}

func TestFeatureExtractorInterface(t *testing.T) {
	// Verify both implementations satisfy the interface
	var _ FeatureExtractor = &MockFeatureExtractor{}
	var _ FeatureExtractor = &LLMFeatureExtractor{}
}
