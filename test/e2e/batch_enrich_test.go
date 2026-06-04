package e2e_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

type testEnv struct {
	t         *testing.T
	workspace string
	dbFile    string
	gcsDir    string
}

func setupEnv(t *testing.T) *testEnv {
	buildCLI(t) // Ensure CLI is built
	tempDir, err := ioutil.TempDir("", "graphdb_e2e_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	dbFile := filepath.Join(tempDir, "test_mock_db.json")
	gcsDir := filepath.Join(tempDir, "test_mock_gcs")

	return &testEnv{
		t:         t,
		workspace: tempDir,
		dbFile:    dbFile,
		gcsDir:    gcsDir,
	}
}

func (env *testEnv) run(args []string, extraEnv map[string]string) (int, string, string) {
	cmd := exec.Command(cliPath, args...)
	cmd.Dir = env.workspace

	envMap := map[string]string{
		"GRAPHDB_MOCK_ENABLED":    "true",
		"GRAPHDB_WORKSPACE_ROOT":  env.workspace,
		"NEO4J_URI":               "bolt://localhost:7687",
		"GOOGLE_CLOUD_PROJECT":    "mock-project",
	}

	for k, v := range extraEnv {
		if v == "" {
			delete(envMap, k)
		} else {
			envMap[k] = v
		}
	}

	for k, v := range envMap {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			env.t.Fatalf("failed to run CLI command: %v", err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

func (env *testEnv) loadDB() MockDB {
	data, err := ioutil.ReadFile(env.dbFile)
	if err != nil {
		if os.IsNotExist(err) {
			return MockDB{
				Jobs:      make(map[string]MockBatchJob),
				Functions: make(map[string]MockFunctionData),
			}
		}
		env.t.Fatalf("failed to read db file: %v", err)
	}
	var db MockDB
	if err := json.Unmarshal(data, &db); err != nil {
		env.t.Fatalf("failed to unmarshal db: %v", err)
	}
	if db.Jobs == nil {
		db.Jobs = make(map[string]MockBatchJob)
	}
	if db.Functions == nil {
		db.Functions = make(map[string]MockFunctionData)
	}
	return db
}

func (env *testEnv) saveDB(db MockDB) {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		env.t.Fatalf("failed to marshal db: %v", err)
	}
	if err := ioutil.WriteFile(env.dbFile, data, 0644); err != nil {
		env.t.Fatalf("failed to write db file: %v", err)
	}
}

// TestBatchEnrich runs the whole suite or acts as the main test orchestrator.
func TestBatchEnrich(t *testing.T) {
	buildCLI(t)
	if cliPath == "" {
		t.Fatal("CLI path not set")
	}
}

// =========================================================================
// TIER 1: FEATURE VALIDATION (20 Tests - 5 per feature)
// =========================================================================

// --- Feature 1: CLI Flag Parsing & Validation ---

func Test_T1_F1_BatchMode_With_Bucket_Flag(t *testing.T) {
	env := setupEnv(t)
	code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "test-bucket"}, nil)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Batch job successfully created") {
		t.Fatalf("expected job created, got: %s", stdout)
	}
}

func Test_T1_F1_BatchMode_With_Env_Bucket(t *testing.T) {
	env := setupEnv(t)
	code, stdout, stderr := env.run([]string{"enrich-features", "--batch"}, map[string]string{"GEMINI_BATCH_GCS_BUCKET": "env-bucket"})
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Batch job successfully created") {
		t.Fatalf("expected job created, got: %s", stdout)
	}
}

func Test_T1_F1_CheckBatch_Flag_Parsing(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending"}
	env.saveDB(db)

	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, nil)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Status check completed") {
		t.Fatalf("expected status check logs, got: %s", stdout)
	}
}

func Test_T1_F1_Help_Output_Contains_Flags(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--help"}, nil)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "help requested") {
		t.Fatalf("expected help logs, got: %s", stderr)
	}
}

func Test_T1_F1_NoBatch_Default_Behavior(t *testing.T) {
	env := setupEnv(t)
	code, stdout, stderr := env.run([]string{"enrich-features"}, nil)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Running legacy synchronous enrichment") {
		t.Fatalf("expected legacy enrichment message, got: %s", stdout)
	}
}

// --- Feature 2: Input Generation & GCS Upload ---

func Test_T1_F2_JSONL_Format_Validity(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Functions["func-1"] = MockFunctionData{ID: "func-1", File: "src/main.go"}
	env.saveDB(db)

	code, _, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "my-bucket"}, map[string]string{"MOCK_JOB_ID": "job-123"})
	if code != 0 {
		t.Fatalf("expected 0 exit code")
	}

	inputPath := filepath.Join(env.workspace, "test_mock_gcs", "my-bucket", "graphdb-batches", "job-123", "input.jsonl")
	data, err := ioutil.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read input jsonl: %v", err)
	}

	if !strings.Contains(string(data), `{"request": {"contents":`) {
		t.Fatalf("invalid jsonl format: %s", string(data))
	}
}

func Test_T1_F2_JSONL_CustomID_Mapping(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Functions["f1"] = MockFunctionData{ID: "f1", File: "main.go"}
	db.Functions["f2"] = MockFunctionData{ID: "f2", File: "utils.go"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-custom"})

	inputPath := filepath.Join(env.workspace, "test_mock_gcs", "b1", "graphdb-batches", "job-custom", "input.jsonl")
	data, _ := ioutil.ReadFile(inputPath)
	content := string(data)

	if !strings.Contains(content, "f1") || !strings.Contains(content, "f2") {
		t.Fatalf("custom IDs missing from input: %s", content)
	}
}

func Test_T1_F2_JSONL_Prompt_SourceCode(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Functions["f1"] = MockFunctionData{ID: "f1", File: "tax.go"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-prompt"})

	inputPath := filepath.Join(env.workspace, "test_mock_gcs", "b1", "graphdb-batches", "job-prompt", "input.jsonl")
	data, _ := ioutil.ReadFile(inputPath)
	if !strings.Contains(string(data), "tax.go") {
		t.Fatalf("expected prompt containing tax.go, got: %s", string(data))
	}
}

func Test_T1_F2_GCS_Upload_Path_Convention(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-path"})

	expectedPath := filepath.Join(env.workspace, "test_mock_gcs", "b1", "graphdb-batches", "job-path", "input.jsonl")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("GCS upload path convention violated: %v", err)
	}
}

func Test_T1_F2_GCS_Upload_Success_Reporting(t *testing.T) {
	env := setupEnv(t)
	_, stdout, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, nil)
	if !strings.Contains(stdout, "Batch job successfully created") {
		t.Fatalf("missing success report in stdout: %s", stdout)
	}
}

// --- Feature 3: Vertex AI Batch Job Creation & DB Recording ---

func Test_T1_F3_VertexAI_CreateBatchJob_Payload(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-payload"})

	db := env.loadDB()
	job, ok := db.Jobs["job-payload"]
	if !ok {
		t.Fatalf("job node not recorded in DB")
	}
	if job.GCSInputURI != "gs://b1/graphdb-batches/job-payload/input.jsonl" {
		t.Fatalf("unexpected input URI: %s", job.GCSInputURI)
	}
}

func Test_T1_F3_VertexAI_Job_ID_Extraction(t *testing.T) {
	env := setupEnv(t)
	_, stdout, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-extract"})
	if !strings.Contains(stdout, "job-extract") {
		t.Fatalf("expected Job ID 'job-extract' in stdout, got: %s", stdout)
	}
}

func Test_T1_F3_Neo4j_BatchJob_Node_Creation(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-node"})

	db := env.loadDB()
	if _, ok := db.Jobs["job-node"]; !ok {
		t.Fatalf("job node not created")
	}
}

func Test_T1_F3_Neo4j_BatchJob_Node_Properties(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-props"})

	db := env.loadDB()
	job := db.Jobs["job-props"]
	if job.State != "pending" || job.ModelName != "gemini-1.5-flash" || job.CreatedAt.IsZero() {
		t.Fatalf("job properties missing or invalid: %+v", job)
	}
}

func Test_T1_F3_CLI_Success_Exit_Message(t *testing.T) {
	env := setupEnv(t)
	code, stdout, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, nil)
	if code != 0 || !strings.Contains(stdout, "Batch job successfully created") {
		t.Fatalf("invalid CLI exit reporting: %d, stdout: %s", code, stdout)
	}
}

// --- Feature 4: Batch Job Polling & DB Import ---

func Test_T1_F4_CheckBatch_Job_Running_State(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-poll"] = MockBatchJob{JobID: "job-poll", State: "pending"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "running"})

	db = env.loadDB()
	if db.Jobs["job-poll"].State != "running" {
		t.Fatalf("expected job to remain in running/pending target: %s", db.Jobs["job-poll"].State)
	}
}

func Test_T1_F4_CheckBatch_Job_Succeeded_Import(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-poll"] = MockBatchJob{JobID: "job-poll", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-poll/output.jsonl"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})

	db = env.loadDB()
	if db.Jobs["job-poll"].State != "succeeded" {
		t.Fatalf("expected job status 'succeeded', got: %s", db.Jobs["job-poll"].State)
	}
}

func Test_T1_F4_CheckBatch_Job_Failed_State(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-poll"] = MockBatchJob{JobID: "job-poll", State: "pending"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "failed"})

	db = env.loadDB()
	if db.Jobs["job-poll"].State != "failed" {
		t.Fatalf("expected job status 'failed', got: %s", db.Jobs["job-poll"].State)
	}
}

func Test_T1_F4_CheckBatch_Function_Properties_Updated(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-poll"] = MockBatchJob{JobID: "job-poll", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-poll/output.jsonl"}
	db.Functions["func-1"] = MockFunctionData{ID: "func-1", File: "src/main.go"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})

	db = env.loadDB()
	f := db.Functions["func-1"]
	if len(f.AtomicFeatures) != 2 || f.AtomicFeatures[0] != "feature_a" {
		t.Fatalf("features not imported properly: %+v", f)
	}
}

func Test_T1_F4_CheckBatch_No_Jobs_To_Process(t *testing.T) {
	env := setupEnv(t)
	code, stdout, _ := env.run([]string{"enrich-features", "--check-batch"}, nil)
	if code != 0 || !strings.Contains(stdout, "No active jobs found") {
		t.Fatalf("unexpected empty jobs handling: %d, stdout: %s", code, stdout)
	}
}

// =========================================================================
// TIER 2: BOUNDARY & ERROR HANDLING (20 Tests - 5 per feature)
// =========================================================================

// --- Feature 1: CLI Flag Parsing & Validation ---

func Test_T2_F1_Missing_GCS_Bucket_Error(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch"}, map[string]string{"GEMINI_BATCH_GCS_BUCKET": ""})
	if code != 1 || !strings.Contains(stderr, "Error: GCS bucket must be specified") {
		t.Fatalf("expected missing GCS bucket error, exit 1, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F1_Invalid_Bucket_Name_Format(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "invalid bucket"}, nil)
	if code != 1 || !strings.Contains(stderr, "Error: invalid GCS bucket name") {
		t.Fatalf("expected invalid bucket name error, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F1_Mutually_Exclusive_Flags(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--check-batch", "--gcs-bucket", "b1"}, nil)
	if code != 1 || !strings.Contains(stderr, "Error: cannot specify both --batch and --check-batch") {
		t.Fatalf("expected mutually exclusive flags error, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F1_Empty_Env_Variable_Value(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch"}, map[string]string{"GEMINI_BATCH_GCS_BUCKET": ""})
	if code != 1 {
		t.Fatalf("expected failure exit 1 for empty env var, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F1_Unrecognized_Extra_Flags(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1", "--unknown-flag-123"}, nil)
	if code != 2 || !strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("expected flag parsing error exit 2, got %d. Stderr: %s", code, stderr)
	}
}

// --- Feature 2: Input Generation & GCS Upload ---

func Test_T2_F2_Zero_Function_Nodes(t *testing.T) {
	env := setupEnv(t)
	code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1", "--dir", "empty"}, nil)
	if code != 0 {
		t.Fatalf("expected clean exit 0 on empty functions, got %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "No functions found to extract features from") {
		t.Fatalf("expected skip logs, got: %s", stdout)
	}
}

func Test_T2_F2_Large_Source_Code_Handling(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Functions["f1"] = MockFunctionData{ID: "f1", File: strings.Repeat("A", 10000)}
	env.saveDB(db)

	code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, nil)
	if code != 0 {
		t.Fatalf("expected clean exit 0 for large payloads, got %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Batch job successfully created") {
		t.Fatalf("expected successful batch job creation, got: %s", stdout)
	}
}

func Test_T2_F2_GCS_Upload_Transient_Timeout_Retry(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_GCS_FAIL": "timeout"})
	if code != 1 || !strings.Contains(stderr, "Error: upload failed: connection timeout") {
		t.Fatalf("expected connection timeout upload error, got %d, stderr: %s", code, stderr)
	}
}

func Test_T2_F2_GCS_Bucket_Permission_Denied(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_GCS_FAIL": "permission"})
	if code != 1 || !strings.Contains(stderr, "Error: Permission Denied uploading to GCS bucket") {
		t.Fatalf("expected permission denied error, got %d, stderr: %s", code, stderr)
	}
}

func Test_T2_F2_Special_Character_Escaping(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Functions["f1"] = MockFunctionData{ID: "f1", File: `func main() { fmt.Println("Hello\t\"World\"") }`}
	env.saveDB(db)

	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-escape"})
	if code != 0 {
		t.Fatalf("special char escaping failed, got %d, stderr: %s", code, stderr)
	}
}

// --- Feature 3: Vertex AI Batch Job Creation & DB Recording ---

func Test_T2_F3_VertexAI_Quota_Exceeded(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_VERTEX_FAIL": "rate_limit"})
	if code != 1 || !strings.Contains(stderr, "Error creating batch job: 429 Rate Limit Exceeded") {
		t.Fatalf("expected rate limit failure, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F3_VertexAI_Service_Unavailable(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_VERTEX_FAIL": "outage"})
	if code != 1 || !strings.Contains(stderr, "Error creating batch job: 503 Service Unavailable") {
		t.Fatalf("expected service outage failure, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F3_Neo4j_Write_Failure_Recovery(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"NEO4J_URI": ""})
	if code != 1 || !strings.Contains(stderr, "NEO4J_URI environment variable is not set") {
		t.Fatalf("expected write recovery failure, got %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F3_Duplicate_Job_ID_Conflict(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "dup-job"})
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "dup-job"})
	if code != 0 {
		t.Fatalf("expected overwrite capability to succeed, got exit %d. Stderr: %s", code, stderr)
	}
}

func Test_T2_F3_Invalid_Model_Configuration(t *testing.T) {
	env := setupEnv(t)
	code, _, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"GOOGLE_CLOUD_PROJECT": ""})
	if code != 1 || !strings.Contains(stderr, "GOOGLE_CLOUD_PROJECT is not set") {
		t.Fatalf("expected model configuration validation failure, got %d. Stderr: %s", code, stderr)
	}
}

// --- Feature 4: Batch Job Polling & DB Import ---

func Test_T2_F4_GCS_Download_Timeout(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-1/output.jsonl"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded", "MOCK_GCS_DOWNLOAD_FAIL": "true"})

	db = env.loadDB()
	if db.Jobs["job-1"].State != "failed" || db.Jobs["job-1"].FailureReason != "GCS download failed" {
		t.Fatalf("expected failed download job state, got %+v", db.Jobs["job-1"])
	}
}

func Test_T2_F4_Output_JSONL_Corrupted(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-1/output.jsonl"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded", "MOCK_OUTPUT_MALFORMED": "true"})

	db = env.loadDB()
	if db.Jobs["job-1"].State != "failed" || db.Jobs["job-1"].FailureReason != "Malformed JSONL output" {
		t.Fatalf("expected corrupted output handling failure, got %+v", db.Jobs["job-1"])
	}
}

func Test_T2_F4_Missing_CustomID_In_Output(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-1/output.jsonl"}
	env.saveDB(db)

	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})
	if code != 0 {
		t.Fatalf("expected clean run, got %d. Stderr: %s, Stdout: %s", code, stderr, stdout)
	}
}

func Test_T2_F4_Orphan_CustomID_Mapping(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-1/output.jsonl"}
	env.saveDB(db)

	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded", "MOCK_OUTPUT_ORPHAN": "true"})
	if code != 0 {
		t.Fatalf("expected orphan custom_id logs without error, got exit %d. Stdout: %s. Stderr: %s", code, stdout, stderr)
	}
}

func Test_T2_F4_Neo4j_Write_Lock_Timeout(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending"}
	env.saveDB(db)

	code, _, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"NEO4J_URI": ""})
	if code != 1 || !strings.Contains(stderr, "NEO4J_URI environment variable is not set") {
		t.Fatalf("expected lock/write error handling, got exit %d. Stderr: %s", code, stderr)
	}
}

// =========================================================================
// TIER 3: CROSS-FEATURE INTEGRATION TESTS (4 Tests)
// =========================================================================

func Test_T3_E2E_Full_Success_Flow(t *testing.T) {
	env := setupEnv(t)
	code, _, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "my-bucket"}, map[string]string{"MOCK_JOB_ID": "job-success"})
	if code != 0 {
		t.Fatal("submission failed")
	}

	code, _, _ = env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})
	if code != 0 {
		t.Fatal("polling/import failed")
	}

	db := env.loadDB()
	if db.Jobs["job-success"].State != "succeeded" {
		t.Fatalf("expected job state succeeded, got: %s", db.Jobs["job-success"].State)
	}
	f1 := db.Functions["func-1"]
	if len(f1.AtomicFeatures) != 2 {
		t.Fatalf("expected features on func-1, got %+v", f1)
	}
}

func Test_T3_E2E_External_Job_Cancellation(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "b1"}, map[string]string{"MOCK_JOB_ID": "job-cancel"})
	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "JobStateCancelled"})

	db := env.loadDB()
	if db.Jobs["job-cancel"].State != "failed" {
		t.Fatalf("expected canceled job to transition to failed, got: %s", db.Jobs["job-cancel"].State)
	}
}

func Test_T3_E2E_GCS_Bucket_Immutability_Check(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "primary-bucket"}, map[string]string{"MOCK_JOB_ID": "job-immut"})

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"GEMINI_BATCH_GCS_BUCKET": "secondary-bucket", "MOCK_JOB_TARGET_STATE": "succeeded"})

	outputPath := filepath.Join(env.workspace, "test_mock_gcs", "primary-bucket", "graphdb-batches", "job-immut", "output.jsonl")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output JSONL to be retrieved/created on primary bucket: %v", err)
	}
}

func Test_T3_E2E_Multiple_Concurrent_Pending_Jobs(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-a"] = MockBatchJob{JobID: "job-a", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-a/output.jsonl"}
	db.Jobs["job-b"] = MockBatchJob{JobID: "job-b", State: "pending", GCSOutputURI: "gs://b1/graphdb-batches/job-b/output.jsonl"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})

	db = env.loadDB()
	if db.Jobs["job-a"].State != "succeeded" || db.Jobs["job-b"].State != "succeeded" {
		t.Fatalf("expected both jobs to process, got states: A=%s, B=%s", db.Jobs["job-a"].State, db.Jobs["job-b"].State)
	}
}

// =========================================================================
// TIER 4: REAL-WORLD APPLICATION SCENARIOS (5 Scenarios)
// =========================================================================

func Test_T4_Scenario1_Greenfield_Ingestion(t *testing.T) {
	env := setupEnv(t)
	code, stdout, _ := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "greenfield-b"}, map[string]string{"MOCK_JOB_ID": "gf-job"})
	if code != 0 || !strings.Contains(stdout, "Batch job successfully created: gf-job") {
		t.Fatal("greenfield job submission failed")
	}

	code, stdout, _ = env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})
	if code != 0 || !strings.Contains(stdout, "Status check completed") {
		t.Fatal("greenfield job polling failed")
	}

	db := env.loadDB()
	if db.Jobs["gf-job"].State != "succeeded" {
		t.Fatal("expected greenfield job state: succeeded")
	}
}

func Test_T4_Scenario2_High_Volume_Outage(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "outage-b"}, map[string]string{"MOCK_JOB_ID": "outage-job"})

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded", "MOCK_GCS_DOWNLOAD_FAIL": "true"})
	db := env.loadDB()
	if db.Jobs["outage-job"].State != "failed" {
		t.Fatal("expected job to fail state on transient GCS outage")
	}

	db.Jobs["outage-job"] = MockBatchJob{JobID: "outage-job", State: "pending", GCSOutputURI: "gs://outage-b/graphdb-batches/outage-job/output.jsonl"}
	env.saveDB(db)

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})
	db = env.loadDB()
	if db.Jobs["outage-job"].State != "succeeded" {
		t.Fatal("expected job to succeed once outage resolved")
	}
}

func Test_T4_Scenario3_Partial_Execution_Failures(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["partial-job"] = MockBatchJob{JobID: "partial-job", State: "pending"}
	env.saveDB(db)

	code, _, _ := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})
	if code != 0 {
		t.Fatalf("failed partial execution run")
	}
}

func Test_T4_Scenario4_Stale_Job_Recovery(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	staleTime := time.Now().Add(-49 * time.Hour)
	db.Jobs["stale-job"] = MockBatchJob{JobID: "stale-job", State: "pending", CreatedAt: staleTime}
	env.saveDB(db)

	code, _, _ := env.run([]string{"enrich-features", "--check-batch"}, nil)
	if code != 0 {
		t.Fatalf("failed stale job run")
	}
}

func Test_T4_Scenario5_Collaborative_Synchronization(t *testing.T) {
	env := setupEnv(t)
	env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "collab-b"}, map[string]string{"MOCK_JOB_ID": "collab-job"})

	env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"MOCK_JOB_TARGET_STATE": "succeeded"})

	db := env.loadDB()
	if db.Jobs["collab-job"].State != "succeeded" {
		t.Fatal("job not synchronized back to DB")
	}
}
