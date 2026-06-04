package e2e_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// Test_T2_F1_AdversarialGCSBucketNames tests bucket name parameters containing injection attempts.
func Test_T2_F1_AdversarialGCSBucketNames(t *testing.T) {
	env := setupEnv(t)

	// List of malicious or abnormal bucket names
	adversarialBuckets := []string{
		"bucket; rm -rf /",
		"bucket && ls",
		"bucket|cat",
		"../../escaped-bucket",
		"bucket-with-spaces ",
		"bucket\nnewline",
	}

	for _, bucket := range adversarialBuckets {
		t.Run(bucket, func(t *testing.T) {
			code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", bucket}, nil)
			// The CLI should reject bucket names with spaces or return exit code 1 due to validation / upload failure.
			// It must NOT execute the injected commands.
			if code == 0 {
				t.Fatalf("Expected non-zero exit code for invalid bucket name %q, got 0. stdout: %s, stderr: %s", bucket, stdout, stderr)
			}
		})
	}
}

// Test_T2_F4_CorruptMockDBFile tests the CLI's resilience when the mock database file is completely corrupted.
func Test_T2_F4_CorruptMockDBFile(t *testing.T) {
	env := setupEnv(t)

	// Write completely invalid/corrupt content to the database file
	err := ioutil.WriteFile(env.dbFile, []byte("{invalid-json-structure"), 0644)
	if err != nil {
		t.Fatalf("failed to write corrupted DB file: %v", err)
	}

	// Run status check. It should recover gracefully, initialize a new DB, and exit 0 (indicating no active jobs).
	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, nil)
	if code != 0 {
		t.Fatalf("Expected clean exit code 0 on corrupted database file recovery, got %d. stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "No active jobs found") {
		t.Fatalf("Expected 'No active jobs found' in stdout after DB recovery, got: %s", stdout)
	}
}

// Test_T2_F3_MissingGCPProject tests validation check for empty GOOGLE_CLOUD_PROJECT variable in batch mode.
func Test_T2_F3_MissingGCPProject(t *testing.T) {
	env := setupEnv(t)
	// Override GOOGLE_CLOUD_PROJECT to empty string
	code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "valid-bucket"}, map[string]string{"GOOGLE_CLOUD_PROJECT": ""})
	if code != 1 {
		t.Fatalf("Expected exit code 1 for missing GOOGLE_CLOUD_PROJECT, got %d. stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "GOOGLE_CLOUD_PROJECT is not set") {
		t.Fatalf("Expected 'GOOGLE_CLOUD_PROJECT is not set' in stderr, got: %s", stderr)
	}
}

// Test_T2_F3_MissingNeo4jURIBatch tests validation check for empty NEO4J_URI in check-batch mode.
func Test_T2_F3_MissingNeo4jURIBatch(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending"}
	env.saveDB(db)

	// Override NEO4J_URI to empty string
	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{"NEO4J_URI": ""})
	if code != 1 {
		t.Fatalf("Expected exit code 1 for missing NEO4J_URI, got %d. stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "NEO4J_URI environment variable is not set") {
		t.Fatalf("Expected 'NEO4J_URI environment variable is not set' in stderr, got: %s", stderr)
	}
}

// Test_T2_F4_WorkspaceRootDoesNotExist checks CLI behavior when workspace root directory is missing.
func Test_T2_F4_WorkspaceRootDoesNotExist(t *testing.T) {
	env := setupEnv(t)
	nonExistentPath := filepath.Join(env.workspace, "does-not-exist-xyz")

	// Run with a non-existent workspace root env variable
	code, stdout, stderr := env.run([]string{"enrich-features", "--batch", "--gcs-bucket", "valid-bucket"}, map[string]string{"GRAPHDB_WORKSPACE_ROOT": nonExistentPath})
	// It should either create the directory, fail gracefully, or default, but not panic.
	if code != 0 && code != 1 {
		t.Fatalf("Unexpected exit code %d for missing workspace root path. stdout: %s, stderr: %s", code, stdout, stderr)
	}
}
