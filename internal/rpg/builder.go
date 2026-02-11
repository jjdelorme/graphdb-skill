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
	// CategoryClusterer enables 3-level hierarchy: Domain -> Category -> Feature.
	// If nil, falls back to 2-level: Domain -> Feature.
	CategoryClusterer Clusterer
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
			if p, ok := fn.Properties["file"].(string); ok {
				if strings.Contains(p, pathPrefix) {
					domainFuncs = append(domainFuncs, fn)
				}
			}
		}
		domainFeature.MemberFunctions = domainFuncs

		if b.CategoryClusterer != nil {
			// 3-level hierarchy: Domain -> Category -> Feature
			allEdges = b.buildThreeLevel(&domainFeature, domainFuncs, name, pathPrefix, allEdges)
		} else {
			// 2-level hierarchy: Domain -> Feature
			allEdges = b.buildTwoLevel(&domainFeature, domainFuncs, name, pathPrefix, allEdges)
		}

		rootFeatures = append(rootFeatures, domainFeature)
	}

	return rootFeatures, allEdges, nil
}

func (b *Builder) buildTwoLevel(domain *Feature, funcs []graph.Node, name, pathPrefix string, allEdges []graph.Edge) []graph.Edge {
	clusters, _ := b.Clusterer.Cluster(funcs, name)
	for clusterName, nodes := range clusters {
		child := &Feature{
			ID:              "feat-" + clusterName,
			Name:            clusterName,
			ScopePath:       pathPrefix,
			MemberFunctions: nodes,
		}

		allEdges = append(allEdges, graph.Edge{
			SourceID: domain.ID,
			TargetID: child.ID,
			Type:     "PARENT_OF",
		})

		for _, fn := range nodes {
			allEdges = append(allEdges, graph.Edge{
				SourceID: fn.ID,
				TargetID: child.ID,
				Type:     "IMPLEMENTS",
			})
		}

		domain.Children = append(domain.Children, child)
	}
	return allEdges
}

func (b *Builder) buildThreeLevel(domain *Feature, funcs []graph.Node, name, pathPrefix string, allEdges []graph.Edge) []graph.Edge {
	// First pass: coarse clustering into categories
	categories, _ := b.CategoryClusterer.Cluster(funcs, name)
	for catName, catNodes := range categories {
		category := &Feature{
			ID:              "cat-" + catName,
			Name:            catName,
			ScopePath:       pathPrefix,
			MemberFunctions: catNodes,
			Children:        make([]*Feature, 0),
		}

		allEdges = append(allEdges, graph.Edge{
			SourceID: domain.ID,
			TargetID: category.ID,
			Type:     "PARENT_OF",
		})

		// Second pass: fine-grained clustering within each category
		features, _ := b.Clusterer.Cluster(catNodes, catName)
		for featName, featNodes := range features {
			feature := &Feature{
				ID:              "feat-" + featName,
				Name:            featName,
				ScopePath:       pathPrefix,
				MemberFunctions: featNodes,
			}

			allEdges = append(allEdges, graph.Edge{
				SourceID: category.ID,
				TargetID: feature.ID,
				Type:     "PARENT_OF",
			})

			for _, fn := range featNodes {
				allEdges = append(allEdges, graph.Edge{
					SourceID: fn.ID,
					TargetID: feature.ID,
					Type:     "IMPLEMENTS",
				})
			}

			category.Children = append(category.Children, feature)
		}

		domain.Children = append(domain.Children, category)
	}
	return allEdges
}
