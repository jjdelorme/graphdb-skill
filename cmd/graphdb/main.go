package main

import (
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/logger"
	"os"
	"strings"
)

// Version is injected at build time
var Version = "dev"

var (
	ingestCmd              = handleIngest
	enrichCmd              = handleEnrichFeatures
	importCmd              = handleImport
	enrichTopologyCmd      = handleEnrichTopology
	enrichHistoryCmd       = handleEnrichHistory
	enrichContaminationCmd = handleEnrichContamination
	enrichTestsCmd         = handleEnrichTests
	setupProviderFn        = setupProvider
)

func main() {
	// Attempt to load .env file from current or parent directories
	_ = config.LoadEnv()

	// Extract global logging flags before subcommand routing
	var logFile string
	var args []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--log=") || strings.HasPrefix(arg, "-log=") {
			parts := strings.SplitN(arg, "=", 2)
			logFile = parts[1]
		} else if (arg == "--log" || arg == "-log") && i+1 < len(os.Args) {
			logFile = os.Args[i+1]
			i++ // skip next arg
		} else {
			args = append(args, arg)
		}
	}

	// Fallback to environment variable
	if logFile == "" {
		logFile = os.Getenv("GRAPHDB_LOG")
	}

	// Configure logging
	logger.Init(logFile)


	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "ingest":
		ingestCmd(cmdArgs)
	case "query":
		handleQuery(cmdArgs)
	case "enrich-features":
		enrichCmd(cmdArgs)
	case "enrich-topology":
		enrichTopologyCmd(cmdArgs)
	case "enrich-contamination":
		handleEnrichContamination(cmdArgs)
	case "enrich-history":
		handleEnrichHistory(cmdArgs)
	case "enrich-tests":
		handleEnrichTests(cmdArgs)
	case "import":
		importCmd(cmdArgs)
	case "serve":
		handleServe(cmdArgs)
	case "build-all":
		handleBuildAll(cmdArgs)
	case "version", "--version", "-v":
		fmt.Printf("graphdb version %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("GraphDB Skill CLI (Version: %s)\n", Version)
	fmt.Println("Usage: graphdb [global options] <command> [options]")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --log <path>           Output standard logs to the specified file (or set GRAPHDB_LOG env var)")
	fmt.Println("\nCommands:")
	fmt.Println("  ingest                 Parse code and generate graph nodes/edges (JSONL)")
	fmt.Println("  enrich-features        Build the RPG (Repository Planning Graph) Intent Layer")
	fmt.Println("  enrich-topology        Detect structural communities using CPM Leiden algorithm")
	fmt.Println("  enrich-contamination   Identify seams and propagate contamination layers")
	fmt.Println("  enrich-history         Analyze git history to find hotspots and co-changes")
	fmt.Println("  enrich-tests           Link tests to production functions")
	fmt.Println("  import                 Import JSONL files into Neo4j")
	fmt.Println("  query                  Query the graph (structural or semantic)")
	fmt.Println("  serve                  Start the HTTP server and D3 visualizer")
	fmt.Println("  build-all              One-shot: Ingest -> Import -> All Enrichment Phases")
	fmt.Println("  version                Show version info")
	fmt.Println("\nRun 'graphdb <command> --help' for command-specific options.")
}
