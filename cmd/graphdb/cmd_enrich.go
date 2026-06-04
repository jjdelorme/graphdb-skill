package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/genai"
	"graphdb/internal/batch"
	"graphdb/internal/config"
	"graphdb/internal/rpg"
	"graphdb/internal/tools/snippet"
)

func handleEnrichFeatures(args []string) {
	fs := flag.NewFlagSet("enrich-features", flag.ExitOnError)
	dirPtr := fs.String("dir", "", "Directory to analyze")
	batchSizePtr := fs.Int("batch-size", 20, "Batch size for LLM feature extraction")
	cfg := config.LoadConfig()
	if *dirPtr != "" {
		cfg.BaseDir = *dirPtr
	}
	*dirPtr = cfg.BaseDir

	llmConcurrencyPtr := fs.Int("llm-concurrency", cfg.LLMConcurrency, "Number of concurrent LLM requests during extraction/summarization")
	embedBatchSizePtr := fs.Int("embed-batch-size", 100, "Batch size for embedding generation")
	seedPtr := fs.Int64("seed", 42, "Seed for deterministic K-Means clustering")
	appContextPtr := fs.String("app-context", "", "Optional path to an OVERVIEW.md or context preamble file")

	batchPtr := fs.Bool("batch", false, "Submit enrichment tasks to Vertex AI Batch Prediction API")
	checkBatchPtr := fs.Bool("check-batch", false, "Poll status of active batch jobs and import completed results")
	gcsBucketPtr := fs.String("gcs-bucket", "", "GCS bucket name")

	fs.Parse(args)

	// Mutual exclusivity check
	if *batchPtr && *checkBatchPtr {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both --batch and --check-batch")
		os.Exit(1)
	}

	// GCS bucket resolution and syntax check
	gcsBucket := *gcsBucketPtr
	if gcsBucket == "" {
		gcsBucket = cfg.GeminiBatchGCSBucket
	}

	if *batchPtr {
		if gcsBucket == "" {
			fmt.Fprintln(os.Stderr, "Error: GCS bucket must be specified via --gcs-bucket or GEMINI_BATCH_GCS_BUCKET environment variable")
			os.Exit(1)
		}
		if !isValidGCSBucketName(gcsBucket) {
			fmt.Fprintln(os.Stderr, "Error: invalid GCS bucket name")
			os.Exit(1)
		}
	}

	if cfg.GoogleCloudLocation == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("GOOGLE_CLOUD_LOCATION is not set. Please set it in your .env file or environment.\n" +
			"Example: export GOOGLE_CLOUD_LOCATION=global")
	}
	loc := cfg.GoogleCloudLocation

	if cfg.GeminiEmbeddingModel == "" {
		cfg.GeminiEmbeddingModel = "gemini-embedding-001"
	}

	if cfg.GeminiGenerativeModel == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("GEMINI_GENERATIVE_MODEL is not set. Please set it in your .env file or environment.\n" +
			"Example: export GEMINI_GENERATIVE_MODEL=gemini-3.1-flash-lite")
	}

	if cfg.GoogleCloudProject == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("GOOGLE_CLOUD_PROJECT is not set. Please set it in your .env file or environment.\n" +
			"Example: export GOOGLE_CLOUD_PROJECT=my-project-id")
	}

	if cfg.Neo4jURI == "" && os.Getenv("GRAPHDB_MOCK_ENABLED") != "true" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	// Load Application Context
	appContext := ""
	if *appContextPtr != "" {
		data, err := os.ReadFile(*appContextPtr)
		if err == nil {
			appContext = string(data)
		} else {
			log.Printf("Warning: Failed to read app-context file %s: %v", *appContextPtr, err)
		}
	} else {
		// Fallback to OVERVIEW.md in the target directory
		data, err := os.ReadFile(filepath.Join(*dirPtr, "OVERVIEW.md"))
		if err == nil {
			appContext = string(data)
		}
	}

	log.Println("Connecting to Graph Database...")
	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	if *batchPtr {
		log.Println("Initializing clients for Batch Submission...")

		// Initialize GCS client
		gcsStorageClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize GCS client: %v", err)
		}
		defer gcsStorageClient.Close()
		gcsClient := batch.NewRealGCSClient(gcsStorageClient)

		// Initialize GenAI client and wrap Batches
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			Project:  cfg.GoogleCloudProject,
			Location: loc,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			log.Fatalf("Failed to initialize GenAI client: %v", err)
		}
		batchClient := batch.NewRealBatchClient(genaiClient.Batches)

		// Fetch unextracted functions
		log.Println("Fetching unextracted functions...")
		nodes, err := provider.GetUnextractedFunctions(10000)
		if err != nil {
			log.Fatalf("Failed to fetch unextracted functions: %v", err)
		}

		if len(nodes) == 0 {
			fmt.Println("No functions found to extract features from.")
			return
		}

		var items []batch.RequestItem
		for _, node := range nodes {
			name, _ := node.Properties["name"].(string)
			file, _ := node.Properties["file"].(string)
			startLineRaw, _ := node.Properties["start_line"]
			endLineRaw, _ := node.Properties["end_line"]

			startLine := 0
			endLine := 0
			if v, ok := startLineRaw.(int64); ok {
				startLine = int(v)
			} else if v, ok := startLineRaw.(int); ok {
				startLine = v
			}
			if v, ok := endLineRaw.(int64); ok {
				endLine = int(v)
			} else if v, ok := endLineRaw.(int); ok {
				endLine = v
			}

			if file == "" || startLine == 0 || endLine == 0 {
				_ = provider.UpdateAtomicFeatures(node.ID, []string{"unknown"}, false)
				continue
			}

			filePath := file
			if !filepath.IsAbs(filePath) && cfg.BaseDir != "" {
				filePath = filepath.Join(cfg.BaseDir, file)
			}

			code, err := snippet.SliceFile(filePath, startLine, endLine)
			if err != nil {
				log.Printf("Warning: failed to slice file %s:%d-%d: %v", filePath, startLine, endLine, err)
				_ = provider.UpdateAtomicFeatures(node.ID, []string{"unreadable_source"}, false)
				continue
			}

			if len(code) > 4000 {
				code = code[:4000] + "\n// ... truncated"
			}

			prompt := "You are analyzing source code to extract atomic feature descriptors.\n\n" +
				"For the function below, generate a list of Verb-Object descriptors that capture what this function does.\n" +
				"Each descriptor should be a concise action phrase like \"validate email\", \"hash password\", \"send notification\".\n\n" +
				"Rules:\n" +
				"- Use lowercase\n" +
				"- Each descriptor should be 2-4 words: a verb followed by the object/target\n" +
				"- Generate 1-5 descriptors depending on function complexity\n" +
				"- Focus on the function's purpose, not implementation details\n" +
				"- Normalize similar concepts (e.g., \"check\" and \"validate\" -> pick one)\n\n" +
				"Return ONLY a JSON array of strings:\n" +
				"[\"descriptor1\", \"descriptor2\"]\n\n" +
				fmt.Sprintf("Function name: %s\n\n%s", name, code)

			items = append(items, batch.RequestItem{
				CustomID: node.ID,
				Prompt:   prompt,
			})
		}

		if len(items) == 0 {
			fmt.Println("No functions found to extract features from.")
			return
		}

		jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
		if customJob := os.Getenv("MOCK_JOB_ID"); customJob != "" {
			jobID = customJob
		}

		gcsInput := fmt.Sprintf("gs://%s/graphdb-batches/%s/input.jsonl", gcsBucket, jobID)
		gcsOutput := fmt.Sprintf("gs://%s/graphdb-batches/%s/output.jsonl", gcsBucket, jobID)

		inputJSONL, err := batch.GenerateRequestsJSONL(items)
		if err != nil {
			log.Fatalf("Failed to generate JSONL: %v", err)
		}

		objectPath := fmt.Sprintf("graphdb-batches/%s/input.jsonl", jobID)
		if err := gcsClient.Upload(ctx, gcsBucket, objectPath, inputJSONL); err != nil {
			fmt.Fprintf(os.Stderr, "Error: upload failed: %v\n", err)
			os.Exit(1)
		}

		jobName, err := batchClient.CreateJob(ctx, cfg.GeminiGenerativeModel, gcsInput, gcsOutput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating batch job: %v\n", err)
			os.Exit(1)
		}

		if err := provider.CreateBatchJobNode(ctx, jobName, cfg.GeminiGenerativeModel, gcsInput, gcsOutput); err != nil {
			log.Fatalf("Failed to create BatchJob node: %v", err)
		}

		fmt.Printf("Batch job successfully created: %s\n", jobID)
		return
	}

	if *checkBatchPtr {
		log.Println("Initializing clients for Batch Checking & Importing...")

		// Initialize GCS client
		gcsStorageClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to initialize GCS client: %v", err)
		}
		defer gcsStorageClient.Close()
		gcsClient := batch.NewRealGCSClient(gcsStorageClient)

		// Initialize GenAI client and wrap Batches
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			Project:  cfg.GoogleCloudProject,
			Location: loc,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			log.Fatalf("Failed to initialize GenAI client: %v", err)
		}
		batchClient := batch.NewRealBatchClient(genaiClient.Batches)

		// Fetch active batch jobs
		log.Println("Fetching active batch jobs...")
		activeJobs, err := provider.GetActiveBatchJobs(ctx)
		if err != nil {
			log.Fatalf("Failed to get active batch jobs: %v", err)
		}

		if len(activeJobs) == 0 {
			fmt.Println("No active jobs found.")
			return
		}

		processedCount := 0
		for _, job := range activeJobs {
			_, state, failureReason, err := batchClient.GetJobStatus(ctx, job.JobID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying Vertex AI job status: %v\n", err)
				continue
			}

			stateUpper := strings.ToUpper(state)
			if stateUpper == "SUCCEEDED" || stateUpper == "JOB_STATE_SUCCEEDED" {
				bucket, prefix := parseGCSURI(job.GCSOutputURI)
				files, err := gcsClient.ListObjects(ctx, bucket, prefix)
				if err != nil {
					log.Printf("Error listing GCS output: %v", err)
					_ = provider.UpdateBatchJobNodeStatus(ctx, job.JobID, "failed", fmt.Sprintf("GCS list failed: %v", err))
					processedCount++
					continue
				}

				var downloadFailed bool
				for _, file := range files {
					if !strings.HasSuffix(file, ".jsonl") {
						continue
					}
					content, err := gcsClient.Download(ctx, bucket, file)
					if err != nil {
						log.Printf("Error downloading output GCS file %s: %v", file, err)
						downloadFailed = true
						break
					}

					items, err := batch.ParseResponsesJSONL(content)
					if err != nil {
						log.Printf("Error parsing responses JSONL: %v", err)
						downloadFailed = true
						break
					}

					for _, item := range items {
						if item.Error != "" {
							log.Printf("Warning: error in response item for %s: %s", item.CustomID, item.Error)
							_ = provider.UpdateAtomicFeatures(item.CustomID, []string{"batch_api_error"}, false)
							continue
						}

						var features []string
						text := item.Text
						text = strings.TrimPrefix(text, "```json")
						text = strings.TrimSuffix(text, "```")
						text = strings.TrimSpace(text)

						if err := json.Unmarshal([]byte(text), &features); err != nil {
							log.Printf("Warning: failed to unmarshal features for %s: %v", item.CustomID, err)
							features = []string{"error_parsing_features"}
						}

						if len(features) == 0 {
							features = []string{"no_features_detected"}
						}

						if err := provider.UpdateAtomicFeatures(item.CustomID, features, false); err != nil {
							log.Printf("Warning: failed to update features for %s: %v", item.CustomID, err)
						}
					}
				}

				if downloadFailed {
					_ = provider.UpdateBatchJobNodeStatus(ctx, job.JobID, "failed", "GCS download or parsing failed")
				} else {
					_ = provider.UpdateBatchJobNodeStatus(ctx, job.JobID, "succeeded", "")
				}
			} else if stateUpper == "FAILED" || stateUpper == "JOB_STATE_FAILED" ||
				stateUpper == "CANCELLED" || stateUpper == "JOB_STATE_CANCELLED" ||
				stateUpper == "EXPIRED" || stateUpper == "JOB_STATE_EXPIRED" {
				reason := failureReason
				if reason == "" {
					reason = fmt.Sprintf("Job ended with state: %s", state)
				}
				_ = provider.UpdateBatchJobNodeStatus(ctx, job.JobID, "failed", reason)
			} else {
				// State is pending, running, active
				_ = provider.UpdateBatchJobNodeStatus(ctx, job.JobID, strings.ToLower(state), "")
			}
			processedCount++
		}

		fmt.Printf("Status check completed. Processed %d jobs.\n", processedCount)
		return
	}

	extractor := setupExtractor(cfg, appContext)
	embedder := setupEmbedder(cfg)
	summarizer := setupSummarizer(cfg, appContext)

	orchestrator := &rpg.Orchestrator{
		Provider:       provider,
		Extractor:      extractor,
		Embedder:       embedder,
		Summarizer:     summarizer,
		Seed:           *seedPtr,
		LLMConcurrency: *llmConcurrencyPtr,
	}

	log.Println("Starting Database-backed Feature Enrichment...")

	// 1. Atomic Feature Extraction
	if err := orchestrator.RunExtraction(*batchSizePtr, *dirPtr); err != nil {
		log.Fatalf("Extraction failed: %v", err)
	}

	// 2. Embedding Generation
	if err := orchestrator.RunEmbedding(*embedBatchSizePtr); err != nil {
		log.Fatalf("Embedding generation failed: %v", err)
	}

	// 3. Out-of-Core Clustering (Topology generation)
	if err := orchestrator.RunClustering(*dirPtr); err != nil {
		log.Fatalf("Clustering failed: %v", err)
	}

	// 4. Summarization
	if err := orchestrator.RunSummarization(*batchSizePtr, *dirPtr); err != nil {
		log.Fatalf("Summarization failed: %v", err)
	}

	log.Println("Feature enrichment completed successfully.")
}

func parseGCSURI(uri string) (string, string) {
	s := strings.TrimPrefix(uri, "gs://")
	idx := strings.Index(s, "/")
	if idx == -1 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

func isValidGCSBucketName(name string) bool {
	if len(name) < 2 || len(name) > 63 {
		return false
	}
	for i, r := range name {
		if i == 0 || i == len(name)-1 {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}
