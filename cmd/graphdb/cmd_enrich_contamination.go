package main

import (
	"flag"
	"graphdb/internal/config"
	"log"
)

func handleEnrichContamination(args []string) {
	fs := flag.NewFlagSet("enrich-contamination", flag.ExitOnError)
	fs.Parse(args)

	cfg := config.LoadConfig()

	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()

	// Guard: Check if is_volatile flags exist
	hasData, err := provider.HasVolatilityData()
	if err != nil {
		log.Fatalf("Failed to check volatility data presence: %v", err)
	}
	if !hasData {
		log.Fatal("Volatility data is missing. Run 'graphdb enrich-features' first to seed volatility via LLM.")
	}

	log.Printf("Propagating volatility UPWARD through the CALLS graph...")
	if err := provider.PropagateVolatility(); err != nil {
		log.Fatalf("Volatility propagation failed: %v", err)
	}

	log.Println("Calculating risk scores for functions based on volatility...")
	if err := provider.CalculateRiskScores(); err != nil {
		log.Fatalf("Risk score calculation failed: %v", err)
	}

	log.Println("Contamination enrichment completed successfully.")
}
