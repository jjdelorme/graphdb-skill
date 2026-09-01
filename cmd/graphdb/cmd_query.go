package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/query"
	"log"
	"os"
	"strings"
)

func handleQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	typePtr := fs.String("type", "", "Query type: search-features, search-similar, hybrid-context, neighbors, impact, globals, coverage, seams, hotspots, explore-domain, what-if, status, semantic-seams, cypher, duplicates, divergence, communities")
	dirPtr := fs.String("dir", "", "Base directory for source files (used by fetch-source)")
	targetPtr := fs.String("target", "", "Target function name or query text (comma-separated for what-if)")
	target2Ptr := fs.String("target2", "", "Second target (e.g. for locate-usage or what-if)")
	depthPtr := fs.Int("depth", 1, "Traversal depth")
	limitPtr := fs.Int("limit", 10, "Result limit")
	cypherPtr := fs.String("cypher", "", "Raw Cypher query string (for -type cypher)")
	summaryPtr := fs.Bool("summary", false, "Output only structural metrics/counts instead of full arrays")
	modulePtr := fs.String("module", ".*", "Module pattern for seams")
	layerPtr := fs.String("layer", "ui", "Contamination layer for seams (ui, db, io, all)")
	similarityPtr := fs.Float64("similarity", 0.5, "Cosine similarity threshold for semantic-seams (lower means more divergent)")
	edgeTypesPtr := fs.String("edge-types", "", "Comma-separated relationship types for traverse")
	directionPtr := fs.String("direction", "outgoing", "Traversal direction: incoming, outgoing, both")

	// Dual-Lens & Divergence flags
	dualLensPtr := fs.Bool("dual-lens", false, "Use Dual-Lens structural CPM Leiden and cut-edge seam ranking")
	minScorePtr := fs.Float64("min-score", 10.0, "Minimum Actionable Seam Score threshold for dual-lens seams")
	maxCutEdgesPtr := fs.Int("max-cut-edges", 4, "Maximum cut-edge threshold for Tier 1 dual-lens seams")
	domainPtr := fs.String("domain", ".*", "Domain name or pattern for divergence query")

	// Embedder args for 'features' type
	locationPtr := fs.String("location", "", "GCP Location")
	modelPtr := fs.String("model", "", "Embedding model name")

	fs.Parse(args)

	cfg := config.LoadConfig()
	if *dirPtr != "" {
		cfg.BaseDir = *dirPtr
	}

	if cfg.BaseDir != "" && cfg.BaseDir != "." {
		if err := os.Chdir(cfg.BaseDir); err != nil {
			log.Fatalf("Failed to change directory to %s: %v", cfg.BaseDir, err)
		}
	}

	if *modelPtr != "" {
		cfg.GeminiEmbeddingModel = *modelPtr
	}
	if *locationPtr != "" {
		cfg.GoogleCloudLocation = *locationPtr
	}

	if cfg.GoogleCloudLocation == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("GOOGLE_CLOUD_LOCATION is not set. Please set it in your .env file or environment.\n" +
			"Example: export GOOGLE_CLOUD_LOCATION=global")
	}

	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()
	var result any

	switch *typePtr {
	case "cypher":
		if *cypherPtr == "" {
			log.Fatal("-cypher is required for 'cypher' type")
		}
		result, err = provider.RunCypher(*cypherPtr)
		if err != nil {
			log.Fatalf("Cypher query failed: %v", err)
		}

	case "duplicates":
		result, err = provider.FindDuplicates(*similarityPtr, *limitPtr)
		if err != nil {
			log.Fatalf("FindDuplicates failed: %v", err)
		}

	case "features": // Alias
		fallthrough
	case "search-features":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-features'")
		}
		embedder := setupEmbedder(cfg)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchFeatures(*targetPtr, embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchFeatures failed: %v", err)
		}

	case "search-similar":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-similar'")
		}
		embedder := setupEmbedder(cfg)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchSimilarFunctions(*targetPtr, embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchSimilarFunctions failed: %v", err)
		}
		case "hybrid-context":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'hybrid-context'")
		}
		// 1. Structural Neighbors (Dependency Layer)
		neighbors, err := provider.GetNeighbors(*targetPtr, *depthPtr, *limitPtr)
		if err != nil {			log.Fatalf("Neighbors lookup failed: %v", err)
		}

		// 2. Semantic Search (Dependency Layer)
		embedder := setupEmbedder(cfg)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Printf("Warning: Embedding failed for hybrid search: %v", err)
		}
		var similar []*query.FeatureResult
		if len(embeddings) > 0 {
			similar, _ = provider.SearchSimilarFunctions(*targetPtr, embeddings[0], *limitPtr)
		}


		result = map[string]interface{}{
			"neighbors": neighbors,
			"similar":   similar,
		}

	case "test-context": // Alias
		fallthrough
	case "neighbors":
		if *targetPtr == "" {
		        log.Fatal("-target is required for 'neighbors'")
		}
		result, err = provider.GetNeighbors(*targetPtr, *depthPtr, *limitPtr)
	case "impact":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'impact'")
		}
		result, err = provider.GetImpact(*targetPtr, *depthPtr)

	case "globals":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'globals'")
		}
		result, err = provider.GetGlobals(*targetPtr)

	case "coverage":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'coverage'")
		}
		result, err = provider.GetCoverage(*targetPtr)

	case "seams":
		if *dualLensPtr {
			result, err = provider.GetDualLensSeams(context.Background(), *modulePtr, *minScorePtr, *maxCutEdgesPtr, *limitPtr)
		} else {
			result, err = provider.GetSeams(*modulePtr, *layerPtr)
		}

	case "divergence":
		result, err = provider.GetDivergence(context.Background(), *domainPtr)

	case "communities":
		result, err = provider.GetCommunities(context.Background(), *limitPtr)

	case "hotspots":
		result, err = provider.GetHotspots(*modulePtr)

	case "locate-usage":
		if *targetPtr == "" || *target2Ptr == "" {
			log.Fatal("-target and -target2 are required for 'locate-usage'")
		}
		result, err = provider.LocateUsage(*targetPtr, *target2Ptr)

	case "fetch-source":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'fetch-source'")
		}
		source, err := provider.FetchSource(*targetPtr)
		if err != nil {
			log.Fatalf("FetchSource failed: %v", err)
		}
		fmt.Print(source) // Print raw source to stdout
		return

	case "explore-domain":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'explore-domain'")
		}
		result, err = provider.ExploreDomain(*targetPtr)

	case "what-if":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'what-if'")
		}
		targets := strings.Split(*targetPtr, ",")
		if *target2Ptr != "" {
			targets = append(targets, *target2Ptr)
		}
		// Trim whitespace and remove empty strings
		var cleanTargets []string
		for _, t := range targets {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				cleanTargets = append(cleanTargets, trimmed)
			}
		}
		result, err = provider.WhatIf(cleanTargets)

	case "traverse":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'traverse'")
		}
		dir := query.Outgoing
		switch strings.ToLower(*directionPtr) {
		case "incoming":
			dir = query.Incoming
		case "both":
			dir = query.Both
		}
		result, err = provider.Traverse(*targetPtr, *edgeTypesPtr, dir, *depthPtr)

	case "status":
		commit, err := provider.GetGraphState()
		if err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
		result = map[string]string{
			"commit": commit,
		}

	case "overview":
		result, err = provider.GetOverview()

	case "semantic-seams":
		result, err = provider.GetSemanticSeams(context.Background(), *similarityPtr)

	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: search-features, search-similar, hybrid-context, neighbors, impact, globals, coverage, seams, hotspots, explore-domain, what-if, status, semantic-seams, divergence, communities", *typePtr)
	}

	if err != nil {
	        log.Fatalf("Query failed: %v", err)
	}

	if *summaryPtr {
	        result = summarizeResult(result)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
	        log.Fatalf("Failed to encode result: %v", err)
	}
}

func summarizeResult(result any) any {
        switch v := result.(type) {
        case *query.NeighborResult:
                if v == nil {
                        return v
                }
                return map[string]any{
                        "node":                     v.Node,
                        "total_dependencies_found": len(v.Dependencies),
                }
        case map[string]interface{}:
                // Handling the hybrid-context result map
                summaryMap := make(map[string]interface{})
                for key, val := range v {
                        if key == "neighbors" {
                                if nr, ok := val.(*query.NeighborResult); ok {
                                        summaryMap[key] = summarizeResult(nr)
                                } else {
                                        summaryMap[key] = val
                                }
                        } else if key == "similar" {
                                if frSlice, ok := val.([]*query.FeatureResult); ok {
                                        summaryMap[key] = map[string]any{
                                                "total_similar_found": len(frSlice),
                                        }
                                } else {
                                        summaryMap[key] = val
                                }
                        } else {
                                summaryMap[key] = val
                        }
                }
                return summaryMap
        default:
                // For other types, return as-is for now, or implement other summaries if needed
                return result
        }
}
