package main

import (
	"flag"
	"fmt"
	"graphdb/internal/analysis/leiden"
	"graphdb/internal/config"
	"graphdb/internal/graph"
	"log"
	"os"
)

func handleEnrichTopology(args []string) {
	fs := flag.NewFlagSet("enrich-topology", flag.ExitOnError)
	dirPtr := fs.String("dir", "", "Codebase directory path")
	gammaPtr := fs.Float64("gamma", 0.0, "CPM resolution parameter (0.0 for adaptive bisection search)")
	minSizePtr := fs.Int("min-size", 30, "Minimum target cluster size for candidate microservice communities")
	maxSizePtr := fs.Int("max-size", 250, "Maximum target cluster size before triggering recursive sub-clustering")
	suppressHubsPtr := fs.Bool("suppress-hubs", true, "Enables inverse-degree logarithmic damping and top-1% hub quarantine")
	seedPtr := fs.Int64("seed", 42, "Random generator seed for deterministic partitioning")
	offlinePtr := fs.Bool("offline", false, "Fast-path execution for zero-token air-gapped indexing directly from in-memory graph")
	quickPtr := fs.Bool("quick", false, "Alias for --offline")

	fs.Parse(args)

	cfg := config.LoadConfig()
	if *dirPtr != "" {
		cfg.BaseDir = *dirPtr
	}
	if *quickPtr {
		*offlinePtr = true
	}

	if cfg.Neo4jURI == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := setupProviderFn(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer provider.Close()

	log.Println("Streaming CodeElement nodes and physical relationships for topological enrichment...")

	// 1. Fetch nodes
	nodeQuery := `
		// Fetch CodeElement Nodes for Topology
		MATCH (n:CodeElement)
		RETURN n.id AS id
	`
	nodeRecords, err := provider.RunCypher(nodeQuery)
	if err != nil {
		log.Fatalf("Failed to fetch nodes for topology enrichment: %v", err)
	}
	var nodeIDs []string
	for _, r := range nodeRecords {
		if id, ok := r["id"].(string); ok && id != "" {
			nodeIDs = append(nodeIDs, id)
		}
	}

	// 2. Fetch structural edges
	edgeQuery := `
		// Fetch Physical Edges for Topology
		MATCH (s:CodeElement)-[r]->(t:CodeElement)
		WHERE type(r) IN ['CALLS', 'CONTAINS', 'INHERITS', 'USES_GLOBAL', 'CO_CHANGED', 'REFERENCES', 'IMPLICIT_SEMANTIC', 'DEFINES', 'DEFINED_IN']
		RETURN s.id AS source, t.id AS target, type(r) AS type
	`
	edgeRecords, err := provider.RunCypher(edgeQuery)
	if err != nil {
		log.Fatalf("Failed to fetch edges for topology enrichment: %v", err)
	}
	var rawEdges []leiden.RawEdge
	for _, r := range edgeRecords {
		src, _ := r["source"].(string)
		tgt, _ := r["target"].(string)
		relType, _ := r["type"].(string)
		if src != "" && tgt != "" {
			rawEdges = append(rawEdges, leiden.RawEdge{
				SourceID: src,
				TargetID: tgt,
				Type:     relType,
			})
		}
	}

	log.Printf("Running CPM Leiden Community Detection across %d nodes and %d edges (seed=%d, gamma=%.4f)...",
		len(nodeIDs), len(rawEdges), *seedPtr, *gammaPtr)

	leidenCfg := leiden.Config{
		Gamma:            *gammaPtr,
		MinCommunitySize: *minSizePtr,
		MaxCommunitySize: *maxSizePtr,
		SuppressHubs:     *suppressHubsPtr,
		RandomSeed:       *seedPtr,
		MaxIterations:    50,
		ResolutionSteps:  8,
		MaxHierDepth:     3,
	}

	engine := leiden.NewEngine(leidenCfg, leiden.DefaultEdgeWeightMatrix())
	partitionResult, err := engine.Partition(nodeIDs, rawEdges)
	if err != nil {
		log.Fatalf("Failed to partition graph: %v", err)
	}

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Structural communities
	for _, comm := range partitionResult.Communities {
		nodes = append(nodes, &graph.Node{
			ID:    comm.ID,
			Label: "StructuralCommunity",
			Properties: map[string]any{
				"id":                  comm.ID,
				"name":                comm.Name,
				"gamma":               comm.Gamma,
				"size":                comm.Size,
				"density":             comm.Density,
				"internal_edge_count": comm.InternalEdgeCount,
				"bpr_avg":             comm.BPRAvg,
			},
		})
		for _, memberID := range comm.NodeIDs {
			edges = append(edges, &graph.Edge{
				SourceID: memberID,
				TargetID: comm.ID,
				Type:     "IN_COMMUNITY",
			})
		}
	}

	// Shared boundaries
	for _, sb := range partitionResult.SharedBoundaries {
		nodes = append(nodes, &graph.Node{
			ID:    sb.NodeID,
			Label: "SharedBoundary",
			Properties: map[string]any{
				"id":                       sb.NodeID,
				"bpr_max":                  sb.BPRMax,
				"boundary_community_count": sb.BoundaryCommunityCount,
			},
		})
		for commID, ratio := range sb.CommunityBPRs {
			edges = append(edges, &graph.Edge{
				SourceID: sb.NodeID,
				TargetID: commID,
				Type:     "BRIDGES",
				Properties: map[string]any{
					"ratio": ratio,
				},
			})
		}
	}

	// Cross cutting hubs
	for _, hub := range partitionResult.CrossCuttingHubs {
		nodes = append(nodes, &graph.Node{
			ID:    hub.NodeID,
			Label: "CrossCuttingHub",
			Properties: map[string]any{
				"id":        hub.NodeID,
				"degree":    hub.Degree,
				"hub_score": hub.HubScore,
			},
		})
		for commID, affinity := range hub.CommunityAffinities {
			edges = append(edges, &graph.Edge{
				SourceID: hub.NodeID,
				TargetID: commID,
				Type:     "INFRASTRUCTURE_OF",
				Properties: map[string]any{
					"affinity": affinity,
				},
			})
		}
	}

	log.Printf("Persisting %d communities, %d shared boundary nodes, and %d hub nodes to Neo4j...",
		len(partitionResult.Communities), len(partitionResult.SharedBoundaries), len(partitionResult.CrossCuttingHubs))

	if err := provider.ClearStructuralTopology(); err != nil {
		log.Fatalf("Failed to clear previous structural topology: %v", err)
	}

	if err := provider.UpdateStructuralTopology(nodes, edges); err != nil {
		log.Fatalf("Failed to persist structural topology: %v", err)
	}

	fmt.Printf("✅ Structural Topology Enrichment complete: %d communities, %d shared boundaries, %d cross-cutting hubs (gamma=%.4f, quality=%.4f)\n",
		len(partitionResult.Communities), len(partitionResult.SharedBoundaries), len(partitionResult.CrossCuttingHubs), partitionResult.Gamma, partitionResult.Quality)
}
