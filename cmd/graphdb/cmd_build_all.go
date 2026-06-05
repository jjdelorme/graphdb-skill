package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleBuildAll(args []string) {
	fs := flag.NewFlagSet("build-all", flag.ExitOnError)
	dirPtr := fs.String("dir", "", "Directory to process")
	nodesPtr := fs.String("nodes", "nodes.jsonl", "Intermediate output file for nodes")
	edgesPtr := fs.String("edges", "edges.jsonl", "Intermediate output file for edges")
	batchPtr := fs.Bool("batch", false, "Submit feature enrichment to Vertex AI Batch API")
	resumePtr := fs.Bool("resume", false, "Resume paused build-all by checking/importing batch features and running remaining phases")
	gcsBucketPtr := fs.String("gcs-bucket", "", "Optional: GCS bucket name for batch API")
	cleanPtr := fs.Bool("clean", false, "Wipe the Neo4j database before building (disables incremental mode)")
	fs.Parse(args)

	// Mutual exclusivity check
	if *batchPtr && *resumePtr {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both --batch and --resume")
		os.Exit(1)
	}
	if *cleanPtr && *resumePtr {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both --clean and --resume")
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	if *dirPtr != "" {
		cfg.BaseDir = *dirPtr
	}
	*dirPtr = cfg.BaseDir

	if *resumePtr {
		fmt.Println("🔄 Resuming paused Build-All sequence...")
		fmt.Println("========================================")

		provider, err := setupProviderFn(cfg)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer provider.Close()

		ctx := context.Background()

		// Verify that batch jobs exist
		totalJobs, err := provider.GetBatchJobCount(ctx)
		if err != nil {
			log.Fatalf("Failed to check batch job count: %v", err)
		}
		if totalJobs == 0 {
			fmt.Fprintln(os.Stderr, "Error: No batch jobs found in the database. Did you run 'build-all --batch' first?")
			os.Exit(1)
		}

		// Run check-batch to pull results and update states
		fmt.Println("Checking status of active batch jobs and importing features...")
		checkArgs := []string{"--check-batch"}
		enrichCmd(checkArgs)

		// Check if any jobs remain active
		activeJobs, err := provider.GetActiveBatchJobs(ctx)
		if err != nil {
			log.Fatalf("Failed to query active batch jobs: %v", err)
		}
		if len(activeJobs) > 0 {
			fmt.Printf("\n⏳ Some batch jobs are still in progress (%d active). Please run 'build-all --resume' again later.\n", len(activeJobs))
			return
		}

		// Run remaining local feature enrichment steps (embeddings, clustering & summarization)
		fmt.Println("\nRunning remaining local feature enrichment steps (embeddings, clustering & summarization)...")
		enrichArgs := []string{"-dir", *dirPtr}
		enrichCmd(enrichArgs)

		fmt.Println("\n✅ Feature enrichment is complete. Resuming remaining build phases...")

		// 4. Enrich History
		fmt.Println("\n[Phase 4/6] Enriching Git History...")
		historyArgs := []string{"-dir", *dirPtr}
		enrichHistoryCmd(historyArgs)

		// 5. Enrich Contamination
		fmt.Println("\n[Phase 5/6] Enriching Contamination/Risk...")
		contaminationArgs := []string{}
		enrichContaminationCmd(contaminationArgs)

		// 6. Enrich Tests
		fmt.Println("\n[Phase 6/6] Linking Tests...")
		testArgs := []string{}
		enrichTestsCmd(testArgs)

		fmt.Println("\n✅ Build-All Sequence Complete!")
		return
	}

	fmt.Println("🚀 Starting GraphDB Build-All Sequence...")
	fmt.Println("========================================")

	isIncremental := false
	stateCommit := ""
	if *cleanPtr {
		fmt.Println("🧹 Clean build requested. Wiping database...")
		provider, err := setupProviderFn(cfg)
		if err != nil {
			log.Fatalf("Failed to connect to database for wipe: %v", err)
		}
		ctx := context.Background()
		if err := provider.WipeDatabase(ctx); err != nil {
			log.Fatalf("Failed to wipe database: %v", err)
		}
		provider.Close()
		fmt.Println("🧹 Database wiped successfully.")
	} else if cfg.Neo4jURI != "" {
		provider, err := setupProviderFn(cfg)
		if err == nil {
			stateCommit, _ = provider.GetGraphState()
			if stateCommit != "" {
				cmd := exec.Command("git", "merge-base", "--is-ancestor", stateCommit, "HEAD")
				cmd.Dir = *dirPtr
				if err := cmd.Run(); err == nil {
					isIncremental = true
					fmt.Printf("\n[Incremental Mode] Auto-detected incremental mode from commit %s\n", stateCommit)

					// Check for actual changes in supported files
					diffCmd := exec.Command("git", "diff", "--name-only", stateCommit+"..HEAD")
					diffCmd.Dir = *dirPtr
					output, err := diffCmd.Output()
					if err == nil {
						relevantChanges := false
						scanner := bufio.NewScanner(bytes.NewReader(output))
						for scanner.Scan() {
							path := scanner.Text()
							ext := filepath.Ext(path)
							switch strings.ToLower(ext) {
							case ".cs", ".java", ".py", ".ts", ".cpp", ".hpp", ".h", ".c", ".cc", ".sql", ".vb", ".asp", ".aspx", ".ascx":
								relevantChanges = true
								break
							}
							if relevantChanges {
								break
							}
						}

						if !relevantChanges {
							fmt.Println("\n✅ No relevant changes detected since last build. Codebase is in sync with graph state.")
							fmt.Println("Skipping further phases. Build-All Sequence Complete!")
							return
						}
					}
				}
			}
			provider.Close()
		}
	}

	// 1. Ingest
	fmt.Println("\n[Phase 1/6] Ingesting Codebase...")
	var ingestArgs []string
	if isIncremental {
		ingestArgs = []string{"-dir", *dirPtr}
	} else {
		ingestArgs = []string{"-dir", *dirPtr, "-nodes", *nodesPtr, "-edges", *edgesPtr}
	}
	ingestCmd(ingestArgs)

	if !isIncremental {
		// 2. Import Structural Graph
		fmt.Println("\n[Phase 2/6] Importing to Neo4j...")
		importArgs1 := []string{"-nodes", *nodesPtr, "-edges", *edgesPtr}
		importCmd(importArgs1)

		// 2.5 Cleanup intermediate files
		fmt.Println("\nCleaning up intermediate JSONL files...")
		if err := os.Remove(*nodesPtr); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to remove %s: %v\n", *nodesPtr, err)
		}
		if err := os.Remove(*edgesPtr); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to remove %s: %v\n", *edgesPtr, err)
		}
	} else {
		fmt.Println("\n[Phase 2/6] Skipping Import (Incremental mode writes directly to DB)...")

		// In incremental mode, we need to manually update the graph state to HEAD
		// because ingest doesn't do it.
		if headCommit, err := getGitCommit(); err == nil && headCommit != "" {
			fmt.Printf("Updating graph state to %s...\n", headCommit)
			driver, err := setupDriver(cfg)
			if err == nil {
				defer driver.Close(context.Background())
				loader, err := setupLoader(context.Background(), cfg, driver)
				if err == nil {
					_ = loader.UpdateGraphState(context.Background(), headCommit, *dirPtr)
				}
			}
		}
	}

	// 3. Enrich Features
	fmt.Println("\n[Phase 3/6] Enriching Features (in-database)...")
	if *batchPtr {
		enrichArgs := []string{"-dir", *dirPtr, "--batch"}
		if *gcsBucketPtr != "" {
			enrichArgs = append(enrichArgs, "--gcs-bucket", *gcsBucketPtr)
		}
		enrichCmd(enrichArgs)
		fmt.Println("\n⏳ Build-All paused after submitting batch enrichment.")
		fmt.Println("Please run 'graphdb build-all --resume' once the batch job completes to import features and finalize the build.")
		return
	} else {
		enrichArgs := []string{"-dir", *dirPtr}
		enrichCmd(enrichArgs)
	}

	// 4. Enrich History
	fmt.Println("\n[Phase 4/6] Enriching Git History...")
	historyArgs := []string{"-dir", *dirPtr}
	enrichHistoryCmd(historyArgs)

	// 5. Enrich Contamination
	fmt.Println("\n[Phase 5/6] Enriching Contamination/Risk...")
	contaminationArgs := []string{}
	enrichContaminationCmd(contaminationArgs)

	// 6. Enrich Tests
	fmt.Println("\n[Phase 6/6] Linking Tests...")
	testArgs := []string{}
	enrichTestsCmd(testArgs)

	fmt.Println("\n✅ Build-All Sequence Complete!")
}
