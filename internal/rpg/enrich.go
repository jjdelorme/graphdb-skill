package rpg

import (
	"graphdb/internal/graph"
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
		if content, ok := fn.Properties["content"].(string); ok {
			// Truncate to save tokens?
			if len(content) > 200 {
				snippets = append(snippets, content[:200]+"...")
			} else {
				snippets = append(snippets, content)
			}
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
