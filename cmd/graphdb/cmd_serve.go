package main

import (
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/ui"
	"log"
	"net/http"
	"os"
)

func handleServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	portPtr := fs.Int("port", 8080, "Port to run the HTTP server on")
	dirPtr := fs.String("dir", "", "Base directory for source files")
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
		fmt.Fprintf(os.Stderr, "Error: GOOGLE_CLOUD_LOCATION is not set. Please set it in your .env file or environment.\n")
		os.Exit(1)
	}

	if cfg.Neo4jURI == "" {
		fmt.Fprintf(os.Stderr, "Error: NEO4J_URI environment variable is not set\n")
		os.Exit(1)
	}

	provider, err := setupProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to Neo4j: %v\n", err)
		os.Exit(1)
	}

	embedder := setupEmbedder(cfg)

	server := ui.NewServer(provider, embedder, cfg, Version)

	addr := fmt.Sprintf(":%d", *portPtr)
	fmt.Printf("Starting GraphDB visualizer at http://localhost:%d\n", *portPtr)
	log.Printf("Starting web visualizer on http://localhost%s\n", addr)
	
	if err := http.ListenAndServe(addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Server failed to start on port %d: %v\n", *portPtr, err)
		log.Fatalf("Server failed: %v", err)
	}
}
