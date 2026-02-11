package rpg

import (
	"graphdb/internal/graph"
	"strings"
)

type DomainDiscoverer interface {
	// Returns a map of DomainName -> PathPrefix
	DiscoverDomains(fileTree string) (map[string]string, error)
}

type Clusterer interface {
	// Clusters nodes into named groups
	Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error)
}

type Builder struct {
	Discoverer DomainDiscoverer
	Clusterer  Clusterer
}

func (b *Builder) Build(rootPath string, functions []graph.Node) ([]Feature, error) {
	domains, err := b.Discoverer.DiscoverDomains(rootPath)
	if err != nil {
		return nil, err
	}

	var rootFeatures []Feature

	for name, pathPrefix := range domains {
		domainFeature := Feature{
			ID:        "domain-" + name,
			Name:      name,
			ScopePath: pathPrefix,
			Children:  make([]*Feature, 0),
		}

		// Filter functions for this domain
		var domainFuncs []graph.Node
		for _, fn := range functions {
			// Naive check: file_path starts with prefix
			if p, ok := fn.Properties["file_path"].(string); ok {
				if strings.Contains(p, pathPrefix) {
					domainFuncs = append(domainFuncs, fn)
				}
			}
		}

		// Cluster them
		clusters, _ := b.Clusterer.Cluster(domainFuncs, name)
		for clusterName, nodes := range clusters {
			child := &Feature{
				ID:   "feat-" + clusterName,
				Name: clusterName,
			}
			// In a real impl, we would associate the 'nodes' (Functions) 
			// with this 'child' (Feature) via "IMPLEMENTS" edges.
			// For this structure-building phase, we just create the hierarchy.
			domainFeature.Children = append(domainFeature.Children, child)
			_ = nodes 
		}
		
		rootFeatures = append(rootFeatures, domainFeature)
	}

	return rootFeatures, nil
}
