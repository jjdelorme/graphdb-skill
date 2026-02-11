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
	"graphdb/internal/loader"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"graphdb/internal/storage"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
	case "import":
		handleImport(os.Args[2:])
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

func setupExtractor(project, location, token string, mock bool) rpg.FeatureExtractor {
	if mock || project == "" {
		if !mock && project == "" {
			log.Println("Using Mock Feature Extractor (no -project provided)")
		} else {
			log.Println("Using Mock Feature Extractor")
		}
		return &rpg.MockFeatureExtractor{}
	}

	if token == "" {
		token = os.Getenv("VERTEX_API_KEY")
	}

	return rpg.NewLLMFeatureExtractor(project, location, &SimpleTokenProvider{TokenString: token})
}

func handleEnrichFeatures(args []string) {
	fs := flag.NewFlagSet("enrich-features", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to analyze")
	projectPtr := fs.String("project", "", "GCP Project ID")
	locationPtr := fs.String("location", "us-central1", "GCP Location")
	mockEmbedPtr := fs.Bool("mock-embedding", false, "Use mock embedding")
	mockExtractPtr := fs.Bool("mock-extraction", false, "Use mock feature extraction")
	tokenPtr := fs.String("token", "", "GCP Access Token")
	inputPtr := fs.String("input", "graph.jsonl", "Input graph file")
	outputPtr := fs.String("output", "rpg.jsonl", "Output file for RPG nodes and edges")
	batchSizePtr := fs.Int("batch-size", 20, "Batch size for LLM feature extraction")
	clusterModePtr := fs.String("cluster-mode", "file", "Clustering mode: 'file' (structural) or 'semantic' (embedding-based)")

	fs.Parse(args)

	log.Println("Starting feature enrichment...")

	// 1. Load Functions from graph.jsonl
	functions, err := loadFunctions(*inputPtr)
	if err != nil {
		log.Fatalf("Failed to load functions: %v", err)
	}
	log.Printf("Loaded %d functions from %s", len(functions), *inputPtr)

	// 2. Extract atomic features per function
	extractor := setupExtractor(*projectPtr, *locationPtr, *tokenPtr, *mockExtractPtr || *mockEmbedPtr)
	log.Printf("Extracting atomic features (batch size: %d)...", *batchSizePtr)
	for i := range functions {
		fn := &functions[i]
		name, _ := fn.Properties["name"].(string)
		code, _ := fn.Properties["content"].(string)

		descriptors, err := extractor.Extract(code, name)
		if err != nil {
			log.Printf("Warning: extraction failed for %s: %v", name, err)
			continue
		}
		fn.Properties["atomic_features"] = descriptors

		if (i+1)%(*batchSizePtr) == 0 {
			log.Printf("  Extracted features for %d/%d functions", i+1, len(functions))
		}
	}
	log.Printf("Extracted atomic features for %d functions", len(functions))

	// 3. Setup Builder
	var clusterer rpg.Clusterer
	switch *clusterModePtr {
	case "semantic":
		embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
		clusterer = &rpg.EmbeddingClusterer{Embedder: embedder}
		log.Println("Using semantic clustering (embedding-based)")
	default:
		clusterer = &rpg.FileClusterer{}
		log.Println("Using file-based clustering")
	}
	builder := &rpg.Builder{
		Discoverer: &rpg.DirectoryDomainDiscoverer{
			BaseDirs: []string{"internal", "pkg", "cmd", "src"},
		},
		Clusterer: clusterer,
	}

	// 4. Build Feature Hierarchy
	features, edges, err := builder.Build(*dirPtr, functions)
	if err != nil {
		log.Fatalf("Failed to build features: %v", err)
	}

	// 5. Setup Enricher
	summarizer := setupSummarizer(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
	embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
	enricher := &rpg.Enricher{
		Client:   summarizer,
		Embedder: embedder,
	}

	// 6. Enrich Features (recursively, using scoped member functions)
	var enrichAll func(f *rpg.Feature)
	enrichAll = func(f *rpg.Feature) {
		if err := enricher.Enrich(f, f.MemberFunctions); err != nil {
			log.Printf("Warning: failed to enrich feature %s: %v", f.Name, err)
		}
		for _, child := range f.Children {
			enrichAll(child)
		}
	}
	for i := range features {
		enrichAll(&features[i])
	}

	// 7. Flatten for Persistence
	nodes, allEdges := rpg.Flatten(features, edges)

	// 8. Persistence (Emit to storage)
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

	// 9. Output (JSON Tree to stdout for debugging/UI)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(features); err != nil {
		log.Fatalf("Failed to encode features: %v", err)
	}
}

func handleImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	nodesPtr := fs.String("nodes", "", "Path to nodes JSONL file")
	edgesPtr := fs.String("edges", "", "Path to edges JSONL file")
	inputPtr := fs.String("input", "", "Path to combined JSONL file (nodes + edges)")
	batchSizePtr := fs.Int("batch-size", 500, "Batch size for insertion")
	cleanPtr := fs.Bool("clean", false, "Wipe database before importing")
	
	fs.Parse(args)

	if *nodesPtr == "" && *edgesPtr == "" && *inputPtr == "" {
		log.Fatal("Either -input or both -nodes and -edges must be provided")
	}

	cfg := config.LoadConfig()
	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(context.Background())

	loader := loader.NewNeo4jLoader(driver, "neo4j") // Default DB name

	ctx := context.Background()

	// 1. Clean Database (Phase 3)
	if *cleanPtr {
		log.Println("Wiping database...")
		if err := loader.Wipe(ctx); err != nil {
			log.Fatalf("Failed to wipe database: %v", err)
		}
	}

	// 2. Apply Constraints
	log.Println("Applying schema constraints...")
	if err := loader.ApplyConstraints(ctx); err != nil {
		log.Printf("Warning: failed to apply constraints: %v", err)
	}

	// 3. Load Nodes
	var nodeFiles []string
	if *inputPtr != "" {
		nodeFiles = append(nodeFiles, *inputPtr)
	}
	if *nodesPtr != "" {
		nodeFiles = append(nodeFiles, *nodesPtr)
	}

	for _, path := range nodeFiles {
		log.Printf("Importing nodes from %s...", path)
		if err := processBatches(path, *batchSizePtr, func(batch []json.RawMessage) error {
			var nodes []graph.Node
			for _, raw := range batch {
				var n graph.Node
				// Try to parse as node. If "type" field exists and is "Function" etc.
				// For combined files, we need to check if it's a node or edge
				var meta map[string]interface{}
				if err := json.Unmarshal(raw, &meta); err != nil {
					continue
				}
				
				// Heuristic: Edges have sourceId/targetId/type
				if _, ok := meta["sourceId"]; ok {
					continue // It's an edge
				}
				
				if err := json.Unmarshal(raw, &n); err == nil {
					nodes = append(nodes, n)
				}
			}
			return loader.BatchLoadNodes(ctx, nodes)
		}); err != nil {
			log.Fatalf("Failed to import nodes: %v", err)
		}
	}

	// 3. Load Edges
	var edgeFiles []string
	if *inputPtr != "" {
		edgeFiles = append(edgeFiles, *inputPtr)
	}
	if *edgesPtr != "" {
		edgeFiles = append(edgeFiles, *edgesPtr)
	}

	for _, path := range edgeFiles {
		log.Printf("Importing edges from %s...", path)
		if err := processBatches(path, *batchSizePtr, func(batch []json.RawMessage) error {
			var edges []graph.Edge
			for _, raw := range batch {
				var e graph.Edge
				var meta map[string]interface{}
				if err := json.Unmarshal(raw, &meta); err != nil {
					continue
				}
				
				if _, ok := meta["sourceId"]; !ok {
					continue // It's a node
				}
				
				if err := json.Unmarshal(raw, &e); err == nil {
					edges = append(edges, e)
				}
			}
			return loader.BatchLoadEdges(ctx, edges)
		}); err != nil {
			log.Fatalf("Failed to import edges: %v", err)
		}
	}
	
	log.Println("Import complete.")

	// 5. Update Graph State (Commit Hash)
	// Try to get current git commit
	if commit, err := getGitCommit(); err == nil && commit != "" {
		log.Printf("Updating graph state with commit %s...", commit)
		if err := loader.UpdateGraphState(ctx, commit); err != nil {
			log.Printf("Warning: failed to update graph state: %v", err)
		}
	}
}

func getGitCommit() (string, error) {
	// Simple git rev-parse HEAD
	// In a real CLI, we might use the git library or exec
	// Since we are inside the repo, exec is fine
	cmd := "git"
	args := []string{"rev-parse", "HEAD"}
	
	out, err := execCommand(cmd, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Wrapper for testing/mocking if needed
var execCommand = func(name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	return c.Output()
}

func processBatches(path string, batchSize int, process func([]json.RawMessage) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var batch []json.RawMessage
	for scanner.Scan() {
		line := scanner.Bytes()
		// Copy slice because scanner reuses it
		item := make([]byte, len(line))
		copy(item, line)
		
		batch = append(batch, item)
		
		if len(batch) >= batchSize {
			if err := process(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	
	if len(batch) > 0 {
		if err := process(batch); err != nil {
			return err
		}
	}
	
	return scanner.Err()
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
	typePtr := fs.String("type", "", "Query type: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams, explore-domain")
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

	case "explore-domain":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'explore-domain'")
		}
		result, err = provider.ExploreDomain(*targetPtr)

	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams, explore-domain", *typePtr)
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
