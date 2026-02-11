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

func (b *Builder) Build(rootPath string, functions []graph.Node) ([]Feature, []graph.Edge, error) {
	domains, err := b.Discoverer.DiscoverDomains(rootPath)
	if err != nil {
		return nil, nil, err
	}

	var rootFeatures []Feature
	var allEdges []graph.Edge

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
			// Naive check: file starts with prefix
			if p, ok := fn.Properties["file"].(string); ok {
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
			
			// Hierarchy: Domain PARENT_OF Cluster
			allEdges = append(allEdges, graph.Edge{
				SourceID: domainFeature.ID,
				TargetID: child.ID,
				Type:     "PARENT_OF",
			})

			// Implementation: Cluster IMPLEMENTS Function
			for _, fn := range nodes {
				allEdges = append(allEdges, graph.Edge{
					SourceID: child.ID,
					TargetID: fn.ID,
					Type:     "IMPLEMENTS",
				})
			}

			domainFeature.Children = append(domainFeature.Children, child)
		}
		
		rootFeatures = append(rootFeatures, domainFeature)
	}

	return rootFeatures, allEdges, nil
}
