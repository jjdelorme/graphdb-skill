package rpg

import (
	"context"
	"encoding/json"
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"strings"

	"google.golang.org/genai"
)

type Summarizer interface {
	Summarize(snippets []string) (string, string, error)
}

type Enricher struct {
	Client   Summarizer
	Embedder embedding.Embedder
}

func (e *Enricher) Enrich(feature *Feature, functions []graph.Node) error {
	var snippets []string
	for _, fn := range functions {
		var snippet string

		// Include atomic features as context if available
		if af, ok := fn.Properties["atomic_features"].([]string); ok && len(af) > 0 {
			snippet = "// Atomic features: " + strings.Join(af, ", ") + "\n"
		}

		if content, ok := fn.Properties["content"].(string); ok {
			if len(content) > 3000 {
				snippet += content[:3000] + "..."
			} else {
				snippet += content
			}
		}

		if snippet != "" {
			snippets = append(snippets, snippet)
		}
		if len(snippets) > 10 {
			break
		}
	}

	name, desc, err := e.Client.Summarize(snippets)
	if err != nil {
		return err
	}

	feature.Name = name
	feature.Description = desc

	// Generate embedding from the description
	if e.Embedder != nil && desc != "" {
		embeddings, err := e.Embedder.EmbedBatch([]string{desc})
		if err != nil {
			return fmt.Errorf("embedding generation failed: %w", err)
		}
		if len(embeddings) > 0 {
			feature.Embedding = embeddings[0]
		}
	}

	return nil
}

type VertexSummarizer struct {
	Client *genai.Client
	Model  string
}

func NewVertexSummarizer(ctx context.Context, projectID, location string) (*VertexSummarizer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &VertexSummarizer{
		Client: client,
		Model:  "gemini-1.5-flash-002",
	}, nil
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

	ctx := context.Background()
	
	resp, err := s.Client.Models.GenerateContent(ctx, s.Model, genai.Text(prompt), nil)
	if err != nil {
		return "", "", fmt.Errorf("generate content failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return "", "", fmt.Errorf("no candidates returned from Vertex AI")
	}
	
	// Check content parts
	cand := resp.Candidates[0]
	if cand.Content == nil || len(cand.Content.Parts) == 0 {
		return "", "", fmt.Errorf("empty content in response")
	}

	responseText := cand.Content.Parts[0].Text
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
