package rpg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"io"
	"net/http"
	"strings"
)

type Summarizer interface {
	Summarize(snippets []string) (string, string, error)
}

type Enricher struct {
	Client Summarizer
}

func (e *Enricher) Enrich(feature *Feature, functions []graph.Node) error {
	var snippets []string
	for _, fn := range functions {
		// Use full content if available, but limited snippets
		if content, ok := fn.Properties["content"].(string); ok {
			// Still truncate individual files to avoid hitting context limits too early
			if len(content) > 1000 {
				snippets = append(snippets, content[:1000]+"...")
			} else {
				snippets = append(snippets, content)
			}
		}
		if len(snippets) > 10 { // Limit number of snippets
			break
		}
	}

	name, desc, err := e.Client.Summarize(snippets)
	if err != nil {
		return err
	}

	feature.Name = name
	feature.Description = desc
	return nil
}

type VertexSummarizer struct {
	ProjectID     string
	Location      string
	Model         string
	TokenProvider embedding.TokenProvider
	HTTPClient    *http.Client
}

func NewVertexSummarizer(projectID, location string, tokenProvider embedding.TokenProvider) *VertexSummarizer {
	return &VertexSummarizer{
		ProjectID:     projectID,
		Location:      location,
		Model:         "gemini-1.5-flash-002",
		TokenProvider: tokenProvider,
		HTTPClient:    &http.Client{},
	}
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (s *VertexSummarizer) Summarize(snippets []string) (string, string, error) {
	if len(snippets) == 0 {
		return "Unknown Feature", "No code snippets provided for analysis.", nil
	}

	prompt := fmt.Sprintf(`You are a technical architect. Below are code snippets from a group of functions. 
Your task is to:
1. Provide a concise, professional name for this "Feature" (e.g., "User Authentication", "Database Migration Service").
2. Provide a 1-2 sentence description of what this feature does.

Return your response in JSON format ONLY:
{"name": "...", "description": "..."}

Code Snippets:
%s`, strings.Join(snippets, "\n---\n"))

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		s.Location, s.ProjectID, s.Location, s.Model)

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
		return "", "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	token, err := s.TokenProvider.Token()
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("vertex AI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", "", fmt.Errorf("no candidates returned from Vertex AI")
	}

	responseText := result.Candidates[0].Content.Parts[0].Text
	// Strip markdown blocks if present
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var summary struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(responseText), &summary); err != nil {
		return "", "", fmt.Errorf("failed to parse LLM response as JSON: %v. Raw: %s", err, responseText)
	}

	return summary.Name, summary.Description, nil
}
