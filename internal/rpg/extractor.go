package rpg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// FeatureExtractor extracts atomic feature descriptors from a single function.
// Each descriptor is a Verb-Object pair (e.g., "validate email", "hash password").
type FeatureExtractor interface {
	Extract(code string, functionName string) ([]string, error)
}

// LLMFeatureExtractor uses a Vertex AI / Gemini model to extract
// atomic Verb-Object feature descriptors from function source code.
type LLMFeatureExtractor struct {
	Client *genai.Client
	Model  string
}

// NewLLMFeatureExtractor creates an LLMFeatureExtractor with defaults.
func NewLLMFeatureExtractor(ctx context.Context, projectID, location string) (*LLMFeatureExtractor, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &LLMFeatureExtractor{
		Client: client,
		Model:  "gemini-1.5-flash-002",
	}, nil
}

func (e *LLMFeatureExtractor) Extract(code string, functionName string) ([]string, error) {
	if code == "" {
		return nil, nil
	}

	// Truncate very long functions to stay within context limits
	if len(code) > 4000 {
		code = code[:4000] + "\n// ... truncated"
	}

	prompt := "You are analyzing source code to extract atomic feature descriptors.\n\n" +
		"For the function below, generate a list of Verb-Object descriptors that capture what this function does.\n" +
		"Each descriptor should be a concise action phrase like \"validate email\", \"hash password\", \"send notification\".\n\n" +
		"Rules:\n" +
		"- Use lowercase\n" +
		"- Each descriptor should be 2-4 words: a verb followed by the object/target\n" +
		"- Generate 1-5 descriptors depending on function complexity\n" +
		"- Focus on the function's purpose, not implementation details\n" +
		"- Normalize similar concepts (e.g., \"check\" and \"validate\" -> pick one)\n\n" +
		"Return ONLY a JSON array of strings:\n" +
		"[\"descriptor1\", \"descriptor2\"]\n\n" +
		fmt.Sprintf("Function name: %s\n\n%s", functionName, code)

	ctx := context.Background()
	
	resp, err := e.Client.Models.GenerateContent(ctx, e.Model, genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("generate content failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned from Vertex AI")
	}

	// Check content parts
	cand := resp.Candidates[0]
	if cand.Content == nil || len(cand.Content.Parts) == 0 {
		return nil, fmt.Errorf("empty content in response")
	}

	responseText := cand.Content.Parts[0].Text
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var descriptors []string
	if err := json.Unmarshal([]byte(responseText), &descriptors); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON array: %v. Raw: %s", err, responseText)
	}

	return descriptors, nil
}

// MockFeatureExtractor returns fixed descriptors for testing.
type MockFeatureExtractor struct{}

func (m *MockFeatureExtractor) Extract(code string, functionName string) ([]string, error) {
	return []string{"process data", "validate input"}, nil
}
