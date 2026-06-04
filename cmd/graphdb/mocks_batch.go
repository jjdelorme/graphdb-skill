//go:build test_mocks

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Mock database structures for batch jobs
type MockBatchJob struct {
	JobID        string    `json:"jobID"`
	State        string    `json:"state"`
	ModelName    string    `json:"modelName"`
	GCSInputURI  string    `json:"gcsInputURI"`
	GCSOutputURI string    `json:"gcsOutputURI"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	FailureReason string   `json:"failureReason,omitempty"`
}

type MockFunctionData struct {
	ID             string   `json:"id"`
	File           string   `json:"file"`
	AtomicFeatures []string `json:"atomic_features,omitempty"`
}

type MockDB struct {
	Jobs      map[string]MockBatchJob     `json:"jobs"`
	Functions map[string]MockFunctionData `json:"functions"`
}

func loadMockDB() MockDB {
	dbFile := filepath.Join(os.Getenv("GRAPHDB_WORKSPACE_ROOT"), "test_mock_db.json")
	if os.Getenv("GRAPHDB_WORKSPACE_ROOT") == "" {
		dbFile = "test_mock_db.json"
	}
	var db MockDB
	data, err := os.ReadFile(dbFile)
	if err != nil {
		db.Jobs = make(map[string]MockBatchJob)
		db.Functions = make(map[string]MockFunctionData)
		// Seed some default functions for testing if empty
		db.Functions["func-1"] = MockFunctionData{ID: "func-1", File: "src/main.go"}
		db.Functions["func-2"] = MockFunctionData{ID: "func-2", File: "src/utils.go"}
		return db
	}
	if err := json.Unmarshal(data, &db); err != nil {
		db.Jobs = make(map[string]MockBatchJob)
		db.Functions = make(map[string]MockFunctionData)
	}
	if db.Jobs == nil {
		db.Jobs = make(map[string]MockBatchJob)
	}
	if db.Functions == nil {
		db.Functions = make(map[string]MockFunctionData)
	}
	return db
}

func saveMockDB(db MockDB) {
	dbFile := filepath.Join(os.Getenv("GRAPHDB_WORKSPACE_ROOT"), "test_mock_db.json")
	if os.Getenv("GRAPHDB_WORKSPACE_ROOT") == "" {
		dbFile = "test_mock_db.json"
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal mock db: %v", err)
	}
	_ = os.WriteFile(dbFile, data, 0644)
}

func handleMockEnrichFeatures(args []string) {
	fs := flag.NewFlagSet("enrich-features", flag.ContinueOnError)
	batchPtr := fs.Bool("batch", false, "Submit to Vertex AI Batch")
	checkBatchPtr := fs.Bool("check-batch", false, "Poll and import batch jobs")
	gcsBucketPtr := fs.String("gcs-bucket", "", "GCS bucket")
	dirPtr := fs.String("dir", ".", "Directory to analyze")
	_ = fs.Int("batch-size", 20, "Batch size")

	// Set output discard to keep tests quiet unless we print explicitly
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(2)
	}

	if *batchPtr || *checkBatchPtr {
		// Validate NEO4J_URI in batch/check-batch
		if os.Getenv("NEO4J_URI") == "" {
			fmt.Fprintln(os.Stderr, "NEO4J_URI environment variable is not set")
			os.Exit(1)
		}

		// Validate GOOGLE_CLOUD_PROJECT
		if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
			fmt.Fprintln(os.Stderr, "GOOGLE_CLOUD_PROJECT is not set. Please set it in your .env file or environment.")
			os.Exit(1)
		}
	}

	if *batchPtr && *checkBatchPtr {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both --batch and --check-batch")
		os.Exit(1)
	}

	// Get GCS Bucket
	gcsBucket := *gcsBucketPtr
	if gcsBucket == "" {
		gcsBucket = os.Getenv("GEMINI_BATCH_GCS_BUCKET")
	}

	if *batchPtr {
		if gcsBucket == "" {
			fmt.Fprintln(os.Stderr, "Error: GCS bucket must be specified via --gcs-bucket or GEMINI_BATCH_GCS_BUCKET environment variable")
			os.Exit(1)
		}
		// Validate bucket name syntax
		if !isValidGCSBucketName(gcsBucket) {
			fmt.Fprintln(os.Stderr, "Error: invalid GCS bucket name")
			os.Exit(1)
		}

		db := loadMockDB()
		if len(db.Functions) == 0 {
			fmt.Println("No functions found to extract features from.")
			return
		}

		// Check if mock dir is empty for specific tests
		if *dirPtr == "empty" {
			fmt.Println("No functions found to extract features from.")
			return
		}

		jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
		// Override JobID for deterministic tests if set
		if customJob := os.Getenv("MOCK_JOB_ID"); customJob != "" {
			jobID = customJob
		}

		gcsInput := fmt.Sprintf("gs://%s/graphdb-batches/%s/input.jsonl", gcsBucket, jobID)
		gcsOutput := fmt.Sprintf("gs://%s/graphdb-batches/%s/output.jsonl", gcsBucket, jobID)

		// Simulate writing JSONL to GCS
		gcsMockDir := filepath.Join(os.Getenv("GRAPHDB_WORKSPACE_ROOT"), "test_mock_gcs", gcsBucket, "graphdb-batches", jobID)
		if os.Getenv("GRAPHDB_WORKSPACE_ROOT") == "" {
			gcsMockDir = filepath.Join("test_mock_gcs", gcsBucket, "graphdb-batches", jobID)
		}
		_ = os.MkdirAll(gcsMockDir, 0755)

		// Simulate permission error
		if os.Getenv("MOCK_GCS_FAIL") == "permission" {
			fmt.Fprintln(os.Stderr, "Error: Permission Denied uploading to GCS bucket")
			os.Exit(1)
		}
		// Simulate network timeout
		if os.Getenv("MOCK_GCS_FAIL") == "timeout" {
			fmt.Fprintln(os.Stderr, "Error: upload failed: connection timeout")
			os.Exit(1)
		}

		// Generate JSONL lines
		var lines []string
		for _, f := range db.Functions {
			line := fmt.Sprintf(`{"request": {"contents": [{"parts": [{"text": "Extract features for %s in function %s"}]}]}}`, f.File, f.ID)
			lines = append(lines, line)
		}
		inputContent := strings.Join(lines, "\n")
		err := os.WriteFile(filepath.Join(gcsMockDir, "input.jsonl"), []byte(inputContent), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing input file: %v\n", err)
			os.Exit(1)
		}

		// Simulate Vertex AI rate limit
		if os.Getenv("MOCK_VERTEX_FAIL") == "rate_limit" {
			fmt.Fprintln(os.Stderr, "Error creating batch job: 429 Rate Limit Exceeded")
			os.Exit(1)
		}
		if os.Getenv("MOCK_VERTEX_FAIL") == "outage" {
			fmt.Fprintln(os.Stderr, "Error creating batch job: 503 Service Unavailable")
			os.Exit(1)
		}

		// Record Job
		job := MockBatchJob{
			JobID:        jobID,
			State:        "pending",
			ModelName:    "gemini-1.5-flash",
			GCSInputURI:  gcsInput,
			GCSOutputURI: gcsOutput,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		db.Jobs[jobID] = job
		saveMockDB(db)

		fmt.Printf("Batch job successfully created: %s\n", jobID)
		return
	}

	if *checkBatchPtr {
		db := loadMockDB()
		if len(db.Jobs) == 0 {
			fmt.Println("No active jobs found.")
			return
		}

		processedCount := 0
		for id, job := range db.Jobs {
			if job.State == "succeeded" || job.State == "failed" {
				continue
			}

			// Simulate get failure
			if os.Getenv("MOCK_VERTEX_GET_FAIL") == "true" {
				fmt.Fprintln(os.Stderr, "Error querying Vertex AI job status: 500 Internal Error")
				continue
			}

			// In mock mode, transition state
			newState := "running"
			if os.Getenv("MOCK_JOB_TARGET_STATE") != "" {
				newState = os.Getenv("MOCK_JOB_TARGET_STATE")
			} else {
				// Default auto-transition if no override
				if job.State == "pending" {
					newState = "running"
				} else if job.State == "running" {
					newState = "succeeded"
				}
			}

			job.State = newState
			job.UpdatedAt = time.Now()

			if newState == "succeeded" {
				// Simulate output downloading and parsing
				if job.GCSOutputURI == "" || !strings.HasPrefix(job.GCSOutputURI, "gs://") {
					job.State = "failed"
					job.FailureReason = "GCS download failed: invalid GCSOutputURI"
					db.Jobs[id] = job
					continue
				}
				parts := strings.Split(job.GCSOutputURI, "/")
				if len(parts) < 5 {
					job.State = "failed"
					job.FailureReason = "GCS download failed: invalid GCSOutputURI format"
					db.Jobs[id] = job
					continue
				}
				bucket := parts[2]
				jobID := parts[4]

				gcsMockDir := filepath.Join(os.Getenv("GRAPHDB_WORKSPACE_ROOT"), "test_mock_gcs", bucket, "graphdb-batches", jobID)
				if os.Getenv("GRAPHDB_WORKSPACE_ROOT") == "" {
					gcsMockDir = filepath.Join("test_mock_gcs", bucket, "graphdb-batches", jobID)
				}

				if os.Getenv("MOCK_GCS_DOWNLOAD_FAIL") == "true" {
					fmt.Fprintln(os.Stderr, "Error downloading output JSONL: GCS file not found")
					job.State = "failed"
					job.FailureReason = "GCS download failed"
					db.Jobs[id] = job
					saveMockDB(db)
					continue
				}

				// Generate mock output JSONL
				_ = os.MkdirAll(gcsMockDir, 0755)
				var outputLines []string
				
				if os.Getenv("MOCK_OUTPUT_MALFORMED") == "true" {
					outputLines = append(outputLines, `{"response": {invalid-json}`)
				} else if os.Getenv("MOCK_OUTPUT_ORPHAN") == "true" {
					outputLines = append(outputLines, `{"response": {"contents": [{"parts": [{"text": "feature_a, feature_b"}]}]}, "custom_id": "nonexistent-func"}`)
				} else {
					for funcID := range db.Functions {
						line := fmt.Sprintf(`{"response": {"contents": [{"parts": [{"text": "feature_a, feature_b"}]}]}, "custom_id": "%s"}`, funcID)
						outputLines = append(outputLines, line)
					}
				}

				err := os.WriteFile(filepath.Join(gcsMockDir, "output.jsonl"), []byte(strings.Join(outputLines, "\n")), 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing output mock: %v\n", err)
				}

				// Read and Parse
				outData, err := os.ReadFile(filepath.Join(gcsMockDir, "output.jsonl"))
				if err != nil {
					job.State = "failed"
					job.FailureReason = "GCS download failed"
				} else {
					// Parse each line
					lines := strings.Split(string(outData), "\n")
					for _, l := range lines {
						if strings.TrimSpace(l) == "" {
							continue
						}
						// Malformed json boundary check
						var parsed map[string]any
						if err := json.Unmarshal([]byte(l), &parsed); err != nil {
							fmt.Fprintf(os.Stderr, "Error parsing JSONL output line: %v\n", err)
							job.State = "failed"
							job.FailureReason = "Malformed JSONL output"
							break
						}
						
						customID, ok := parsed["custom_id"].(string)
						if !ok || customID == "" {
							log.Println("Warning: Output line missing custom_id")
							continue
						}

						f, exists := db.Functions[customID]
						if !exists {
							log.Printf("Warning: function %s not found in Neo4j", customID)
							continue
						}

						// Update function properties
						f.AtomicFeatures = []string{"feature_a", "feature_b"}
						db.Functions[customID] = f
					}
				}
			} else if newState == "failed" || newState == "JobStateFailed" || newState == "JobStateCancelled" {
				job.State = "failed"
				job.FailureReason = "Vertex API Job Execution Failed"
			}

			db.Jobs[id] = job
			processedCount++
		}

		saveMockDB(db)
		fmt.Printf("Status check completed. Processed %d jobs.\n", processedCount)
		return
	}

	// Default fallback (synchronous mode)
	handleEnrichFeaturesOriginal(args)
}

func handleEnrichFeaturesOriginal(args []string) {
	fmt.Println("Running legacy synchronous enrichment")
	handleEnrichFeatures(args)
}

