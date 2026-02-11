package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/ingest"
	"graphdb/internal/query"
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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: graphdb <command> [options]")
		fmt.Println("Commands: ingest, query")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ingest":
		handleIngest(os.Args[2:])
	case "query":
		handleQuery(os.Args[2:])
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
	dirPtr := fs.String("dir", ".", "Directory to walk")
	workersPtr := fs.Int("workers", 4, "Number of workers")
	outputPtr := fs.String("output", "graph.jsonl", "Output file path")
	projectPtr := fs.String("project", "", "GCP Project ID for Vertex AI")
	locationPtr := fs.String("location", "us-central1", "GCP Location for Vertex AI")
	mockEmbedPtr := fs.Bool("mock-embedding", false, "Use mock embedding instead of Vertex AI")
	tokenPtr := fs.String("token", "", "GCP Access Token")

	fs.Parse(args)

	// Setup Emitter
	outFile, err := os.Create(*outputPtr)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outFile.Close()
	
	emitter := storage.NewJSONLEmitter(outFile)

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
	log.Printf("Starting walk on %s with %d workers...", *dirPtr, *workersPtr)
	
	if err := walker.Run(ctx, *dirPtr); err != nil {
		log.Fatalf("Walker failed: %v", err)
	}

	log.Printf("Done in %v. Output written to %s", time.Since(start), *outputPtr)
}

func handleQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	typePtr := fs.String("type", "", "Query type: features, neighbors, impact, globals, seams")
	targetPtr := fs.String("target", "", "Target function name or query text")
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
	// Validation for non-mock scenarios or if connection is mandatory
	// For now, we assume connection is mandatory for query
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
	case "features":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'features'")
		}
		embedder := setupEmbedder(*projectPtr, *locationPtr, *tokenPtr, *mockEmbedPtr)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			 log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchFeatures(embeddings[0], *limitPtr)
		
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
		
	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: features, neighbors, impact, globals, seams", *typePtr)
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
