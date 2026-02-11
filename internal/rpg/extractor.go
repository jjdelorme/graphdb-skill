package rpg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"graphdb/internal/embedding"
	"io"
	"net/http"
	"strings"
)

// FeatureExtractor extracts atomic feature descriptors from a single function.
// Each descriptor is a Verb-Object pair (e.g., "validate email", "hash password").
type FeatureExtractor interface {
	Extract(code string, functionName string) ([]string, error)
}

// LLMFeatureExtractor uses a Vertex AI / Gemini model to extract
// atomic Verb-Object feature descriptors from function source code.
type LLMFeatureExtractor struct {
	ProjectID     string
	Location      string
	Model         string
	TokenProvider embedding.TokenProvider
	HTTPClient    *http.Client
}

// NewLLMFeatureExtractor creates an LLMFeatureExtractor with defaults.
func NewLLMFeatureExtractor(projectID, location string, tokenProvider embedding.TokenProvider) *LLMFeatureExtractor {
	return &LLMFeatureExtractor{
		ProjectID:     projectID,
		Location:      location,
		Model:         "gemini-1.5-flash-002",
		TokenProvider: tokenProvider,
		HTTPClient:    &http.Client{},
	}
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

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		e.Location, e.ProjectID, e.Location, e.Model)

	payload := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	token, err := e.TokenProvider.Token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vertex AI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates returned from Vertex AI")
	}

	responseText := result.Candidates[0].Content.Parts[0].Text
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
