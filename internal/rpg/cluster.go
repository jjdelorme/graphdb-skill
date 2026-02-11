package rpg

import (
	"graphdb/internal/graph"
	"path/filepath"
)

type FileClusterer struct{}

func (c *FileClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	clusters := make(map[string][]graph.Node)

	for _, node := range nodes {
		filePath, ok := node.Properties["file"].(string)
		if !ok {
			// Fallback if no file path
			clusters["misc"] = append(clusters["misc"], node)
			continue
		}

		// Use the base filename (without extension) as the cluster name
		name := filepath.Base(filePath)
		ext := filepath.Ext(name)
		name = name[:len(name)-len(ext)]

		clusters[name] = append(clusters[name], node)
	}

	return clusters, nil
}
