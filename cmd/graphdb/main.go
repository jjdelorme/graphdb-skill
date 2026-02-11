package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/ingest"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"graphdb/internal/storage"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// SimpleTokenProvider implements embedding.TokenProvider
type SimpleTokenProvider struct {
	TokenString string
}

func (p *SimpleTokenProvider) Token() (string, error) {
	if p.TokenString == "" {
		return "", fmt.Errorf("no token provided")
	}
	return p.TokenString, nil
}

// MockEmbedder for testing/dry-run
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768) // Dummy 768-dim vector
	}
	return res, nil
}

// SimpleDomainDiscoverer for placeholder RPG
type SimpleDomainDiscoverer struct{}

func (d *SimpleDomainDiscoverer) DiscoverDomains(fileTree string) (map[string]string, error) {
	// Placeholder: returns a single root domain
	return map[string]string{"root": ""}, nil
}

// SimpleClusterer for placeholder RPG
type SimpleClusterer struct{}

func (c *SimpleClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	// Placeholder: puts all nodes in a single "default" cluster
	return map[string][]graph.Node{"default": nodes}, nil
}

// MockSummarizer for placeholder RPG
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(snippets []string) (string, string, error) {
	return "Mock Feature", "Automatically generated description based on " + fmt.Sprintf("%d", len(snippets)) + " snippets.", nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: graphdb <command> [options]")
		fmt.Println("Commands: ingest, query, enrich-features")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ingest":
		handleIngest(os.Args[2:])
	case "query":
		handleQuery(os.Args[2:])
	case "enrich-features":
		handleEnrichFeatures(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func setupEmbedder(project, location, token string, mock bool) embedding.Embedder {
	if mock || project == "" {
		if !mock && project == "" {
			log.Println("Using Mock Embedder (no -project provided)")
		} else {
			log.Println("Using Mock Embedder")
		}
		return &MockEmbedder{}
	}
	
	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	
	return embedding.NewVertexEmbedder(project, location, &SimpleTokenProvider{TokenString: token})
}

func handleIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to walk (ignored if -file-list is used)")
	fileListPtr := fs.String("file-list", "", "Path to a file containing a list of files to process")
	workersPtr := fs.Int("workers", 4, "Number of workers")
	outputPtr := fs.String("output", "graph.jsonl", "Output file path (combined)")
	nodesPtr := fs.String("nodes", "", "Output file path for nodes")
	edgesPtr := fs.String("edges", "", "Output file path for edges")
	projectPtr := fs.String("project", "", "GCP Project ID for Vertex AI")
	locationPtr := fs.String("location", "us-central1", "GCP Location for Vertex AI")
	mockEmbedPtr := fs.Bool("mock-embedding", false, "Use mock embedding instead of Vertex AI")
	tokenPtr := fs.String("token", "", "GCP Access Token")

	fs.Parse(args)

	var emitter storage.Emitter
	if *nodesPtr != "" || *edgesPtr != "" {
		if *nodesPtr == "" || *edgesPtr == "" {
			log.Fatalf("Both -nodes and -edges must be provided for split output")
		}
		nodeFile, err := os.Create(*nodesPtr)
		if err != nil {
			log.Fatalf("Failed to create nodes file: %v", err)
		}
		edgeFile, err := os.Create(*edgesPtr)
		if err != nil {
			log.Fatalf("Failed to create edges file: %v", err)
		}
		emitter = storage.NewSplitJSONLEmitter(nodeFile, edgeFile)
	} else {
		// Setup Combined Emitter
		outFile, err := os.Create(*outputPtr)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		emitter = storage.NewJSONLEmitter(outFile)
	}
	defer emitter.Close()

	// Setup Embedder
	embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)

	// Setup Walker
	walker := ingest.NewWalker(*workersPtr, embedder, emitter)

	// Context with Cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal...")
		cancel()
	}()

	// Run
	start := time.Now()
	
	if *fileListPtr != "" {
		log.Printf("Starting ingestion from file list %s with %d workers...", *fileListPtr, *workersPtr)
		file, err := os.Open(*fileListPtr)
		if err != nil {
			log.Fatalf("Failed to open file list: %v", err)
		}
		defer file.Close()

		walker.WorkerPool.Start()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				walker.WorkerPool.Submit(path)
			}
		}
		walker.WorkerPool.Stop()
	} else {
		log.Printf("Starting walk on %s with %d workers...", *dirPtr, *workersPtr)
		if err := walker.Run(ctx, *dirPtr); err != nil {
			log.Fatalf("Walker failed: %v", err)
		}
	}

	log.Printf("Done in %v.", time.Since(start))
}

func setupSummarizer(project, location, token string, mock bool) rpg.Summarizer {
	if mock || project == "" {
		if !mock && project == "" {
			log.Println("Using Mock Summarizer (no -project provided)")
		} else {
			log.Println("Using Mock Summarizer")
		}
		return &MockSummarizer{}
	}
	
	if token == "" {
		token = os.Getenv("VERTEX_API_KEY") // Fallback
	}
	
	return rpg.NewVertexSummarizer(project, location, &SimpleTokenProvider{TokenString: token})
}

func handleEnrichFeatures(args []string) {
	fs := flag.NewFlagSet("enrich-features", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to analyze")
	projectPtr := fs.String("project", "", "GCP Project ID")
	locationPtr := fs.String("location", "us-central1", "GCP Location")
	mockEmbedPtr := fs.Bool("mock-embedding", false, "Use mock embedding")
	tokenPtr := fs.String("token", "", "GCP Access Token")
	inputPtr := fs.String("input", "graph.jsonl", "Input graph file")
	outputPtr := fs.String("output", "rpg.jsonl", "Output file for RPG nodes and edges")

	fs.Parse(args)

	log.Println("Starting feature enrichment...")

	// 1. Load Functions from graph.jsonl
	functions, err := loadFunctions(*inputPtr)
	if err != nil {
		log.Fatalf("Failed to load functions: %v", err)
	}
	log.Printf("Loaded %d functions from %s", len(functions), *inputPtr)

	// 2. Setup Builder
	builder := &rpg.Builder{
		Discoverer: &rpg.DirectoryDomainDiscoverer{
			BaseDirs: []string{"internal", "pkg", "cmd", "src"},
		},
		Clusterer: &rpg.FileClusterer{},
	}

	// 3. Build Feature Hierarchy
	features, edges, err := builder.Build(*dirPtr, functions)
	if err != nil {
		log.Fatalf("Failed to build features: %v", err)
	}

	// 4. Setup Enricher
	summarizer := setupSummarizer(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
	enricher := &rpg.Enricher{
		Client: summarizer,
	}

	// 5. Enrich Features
	for i := range features {
		if err := enricher.Enrich(&features[i], functions); err != nil {
			log.Printf("Warning: failed to enrich feature %s: %v", features[i].Name, err)
		}
	}

	// 6. Flatten for Persistence
	nodes, allEdges := rpg.Flatten(features, edges)

	// 7. Persistence (Emit to storage)
	outFile, err := os.Create(*outputPtr)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outFile.Close()
	emitter := storage.NewJSONLEmitter(outFile)
	defer emitter.Close()

	for i := range nodes {
		if err := emitter.EmitNode(&nodes[i]); err != nil {
			log.Printf("Warning: failed to emit node: %v", err)
		}
	}
	for i := range allEdges {
		if err := emitter.EmitEdge(&allEdges[i]); err != nil {
			log.Printf("Warning: failed to emit edge: %v", err)
		}
	}

	log.Printf("Successfully emitted %d nodes and %d edges to %s", len(nodes), len(allEdges), *outputPtr)

	// 8. Output (JSON Tree to stdout for debugging/UI)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(features); err != nil {
		log.Fatalf("Failed to encode features: %v", err)
	}
}

func loadFunctions(path string) ([]graph.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var nodes []graph.Node
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var raw map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		
		// Check if it is a node and a Function
		if typeVal, ok := raw["type"].(string); ok && typeVal == "Function" {
			// Reconstruct node
			id, _ := raw["id"].(string)
			node := graph.Node{
				ID:         id,
				Label:      "Function",
				Properties: raw,
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, scanner.Err()
}

func handleQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	typePtr := fs.String("type", "", "Query type: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams")
	targetPtr := fs.String("target", "", "Target function name or query text")
	target2Ptr := fs.String("target2", "", "Second target (e.g. for locate-usage)")
	depthPtr := fs.Int("depth", 1, "Traversal depth")
	limitPtr := fs.Int("limit", 10, "Result limit")
	modulePtr := fs.String("module", ".*", "Module pattern for seams")
	
	// Embedder args for 'features' type
	projectPtr := fs.String("project", "", "GCP Project ID")
	locationPtr := fs.String("location", "us-central1", "GCP Location")
	mockEmbedPtr := fs.Bool("mock-embedding", false, "Use mock embedding")
	tokenPtr := fs.String("token", "", "GCP Access Token")

	fs.Parse(args)

	cfg := config.LoadConfig()
	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := query.NewNeo4jProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()

	var result any

	switch *typePtr {
	case "features": // Alias
		fallthrough
	case "search-features":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-features'")
		}
		embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			 log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchFeatures(embeddings[0], *limitPtr)

	case "search-similar":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-similar'")
		}
		embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			 log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)

	case "hybrid-context":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'hybrid-context'")
		}
		// 1. Structural Neighbors (Dependency Layer)
		neighbors, err := provider.GetNeighbors(*targetPtr, *depthPtr)
		if err != nil {
			log.Fatalf("Neighbors lookup failed: %v", err)
		}

		// 2. Semantic Search (Dependency Layer)
		embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Printf("Warning: Embedding failed for hybrid search: %v", err)
		}
		
		var similar []*query.FeatureResult
		if len(embeddings) > 0 {
			similar, _ = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)
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
		result, err = provider.GetNeighbors(*targetPtr, *depthPtr)
		
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
		
	case "seams":
		result, err = provider.GetSeams(*modulePtr)

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

	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams", *typePtr)
	}
	
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatalf("Failed to encode result: %v", err)
	}
}
