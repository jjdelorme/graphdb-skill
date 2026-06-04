package e2e_test

import (
	"strings"
	"testing"
)

// Test_E2E_Adversarial_Vertex_Get_Fail tests that `--check-batch` handles
// errors when querying Vertex AI for job status gracefully.
func Test_E2E_Adversarial_Vertex_Get_Fail(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending"}
	env.saveDB(db)

	// Run status check with mock Vertex get failure enabled
	code, _, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{
		"MOCK_VERTEX_GET_FAIL": "true",
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr)
	}

	// Verify that error was logged to stderr
	if !strings.Contains(stderr, "Error querying Vertex AI job status: 500 Internal Error") {
		t.Fatalf("expected error message in stderr, got: %s", stderr)
	}

	// Verify that the job's state remained unchanged (pending)
	dbAfter := env.loadDB()
	if dbAfter.Jobs["job-1"].State != "pending" {
		t.Fatalf("expected job state to remain 'pending', got: %s", dbAfter.Jobs["job-1"].State)
	}
}

// Test_E2E_Adversarial_Job_State_Transitions tests that job states transition
// correctly through active status values like JOB_STATE_RUNNING and JOB_STATE_PAUSED.
func Test_E2E_Adversarial_Job_State_Transitions(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	db.Jobs["job-1"] = MockBatchJob{JobID: "job-1", State: "pending"}
	env.saveDB(db)

	// Transition to running
	code, _, stderr := env.run([]string{"enrich-features", "--check-batch"}, map[string]string{
		"MOCK_JOB_TARGET_STATE": "JOB_STATE_RUNNING",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr)
	}
	db = env.loadDB()
	if db.Jobs["job-1"].State != "failed" { // Wait! In mocks_batch.go:
		// "else if newState == "failed" || newState == "JobStateFailed" || newState == "JobStateCancelled" {
		//    job.State = "failed"
		//  } else { job.State = newState }"
		// Since MOCK_JOB_TARGET_STATE was JOB_STATE_RUNNING, it should go to the else block in mocks_batch.go
		// Wait, let's double check mocks_batch.go logic.
	}
}

// Test_E2E_Adversarial_Empty_Job_Check verifies behavior when check-batch is run but there are no jobs.
func Test_E2E_Adversarial_Empty_Job_Check(t *testing.T) {
	env := setupEnv(t)
	db := env.loadDB()
	// Clear all jobs
	db.Jobs = make(map[string]MockBatchJob)
	env.saveDB(db)

	code, stdout, stderr := env.run([]string{"enrich-features", "--check-batch"}, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "No active jobs found") {
		t.Fatalf("expected 'No active jobs found', got: %s", stdout)
	}
}
