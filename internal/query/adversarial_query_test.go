//go:build integration

package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
)

// TestNeo4jBatch_AdversarialContext tests query execution under cancelled contexts and timeouts.
func TestNeo4jBatch_AdversarialContext(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	// 1. Cancelled Context Test
	t.Run("Cancelled Context CreateBatchJobNode", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := p.CreateBatchJobNode(cancelCtx, "test-job-cancelled", "gemini-embedding-001", "gs://in", "gs://out")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "canceled") || strings.Contains(err.Error(), "cancelled") || strings.Contains(err.Error(), "context"), "Expected context cancellation error, got: %v", err)
	})

	// 2. Timeout Context Test
	t.Run("Timeout Context GetActiveBatchJobs", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond) // Ensure timeout has passed

		_, err := p.GetActiveBatchJobs(timeoutCtx)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "canceled"), "Expected deadline exceeded error, got: %v", err)
	})
}

// TestNeo4jBatch_AdversarialInputs tests edge cases like non-existent job ID updates,
// malformed inputs, and extremely large payloads.
func TestNeo4jBatch_AdversarialInputs(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	ctx := context.Background()

	// Ensure cleanup
	defer func() {
		cleanupJobs(p)
	}()
	cleanupJobs(p)

	// 1. Non-existent Job Status Update
	t.Run("Update Non-existent Job Status", func(t *testing.T) {
		// Should not return an error (Cypher MATCH yields empty result, which is a success)
		err := p.UpdateBatchJobNodeStatus(ctx, "non-existent-job-xyz", "succeeded", "")
		assert.NoError(t, err)
	})

	// 2. Extremely large values (1MB strings for inputs, outputs, states, failure reasons)
	t.Run("Extremely Large Values In Create and Update", func(t *testing.T) {
		largeID := "test-job-large-" + strings.Repeat("A", 10000)
		largeModel := strings.Repeat("M", 5000)
		largeInput := "gs://" + strings.Repeat("I", 10000) + "/input"
		largeOutput := "gs://" + strings.Repeat("O", 10000) + "/output"
		largeState := strings.Repeat("S", 1000)
		largeReason := strings.Repeat("R", 50000)

		// Create
		err := p.CreateBatchJobNode(ctx, largeID, largeModel, largeInput, largeOutput)
		assert.NoError(t, err)

		// Update status
		err = p.UpdateBatchJobNodeStatus(ctx, largeID, largeState, largeReason)
		assert.NoError(t, err)

		// Fetch and verify it can be read back cleanly
		activeJobs, err := p.GetActiveBatchJobs(ctx)
		assert.NoError(t, err)

		found := false
		for _, job := range activeJobs {
			if job.JobID == largeID {
				found = true
				assert.Equal(t, largeModel, job.ModelName)
				assert.Equal(t, largeInput, job.GCSInputURI)
				assert.Equal(t, largeOutput, job.GCSOutputURI)
				assert.Equal(t, largeState, job.State)
				assert.Equal(t, largeReason, job.FailureReason)
			}
		}
		// Since state is "largeState", it should be active (not in succeeded/failed terminal list)
		assert.True(t, found, "Job with large parameters should be found in active list")
	})

	// 3. Special characters in IDs and state names
	t.Run("Special Characters In BatchJob Properties", func(t *testing.T) {
		specID := "test-job-spec-`\"'\\;!@#$%^&*()_+{}|:<>?-=[]"
		specModel := "gemini/embedding/test@model"
		specInput := "gs://bucket-name/path-with-'\"`special_char/input.jsonl"
		specOutput := "gs://bucket-name/path-with-'\"`special_char/output/"
		specState := "active-state-!@#"
		specReason := "failed: 'connection lost' \"error_code: 500\""

		err := p.CreateBatchJobNode(ctx, specID, specModel, specInput, specOutput)
		assert.NoError(t, err)

		err = p.UpdateBatchJobNodeStatus(ctx, specID, specState, specReason)
		assert.NoError(t, err)

		activeJobs, err := p.GetActiveBatchJobs(ctx)
		assert.NoError(t, err)

		found := false
		for _, job := range activeJobs {
			if job.JobID == specID {
				found = true
				assert.Equal(t, specModel, job.ModelName)
				assert.Equal(t, specInput, job.GCSInputURI)
				assert.Equal(t, specOutput, job.GCSOutputURI)
				assert.Equal(t, specState, job.State)
				assert.Equal(t, specReason, job.FailureReason)
			}
		}
		assert.True(t, found, "Job with special characters should be found in active list")
	})
}

// Helper function to clean up test jobs
func cleanupJobs(p *Neo4jProvider) {
	ctx := context.Background()
	_, _ = neo4j.ExecuteQuery(ctx, p.driver, `
		MATCH (j:BatchJob) WHERE j.jobID STARTS WITH 'test-job-' DETACH DELETE j
	`, nil, neo4j.EagerResultTransformer)
}

// TestParseBatchJobTime_Adversarial tests the private parseBatchJobTime helper function.
func TestParseBatchJobTime_Adversarial(t *testing.T) {
	// 1. nil value
	t1, err := parseBatchJobTime(nil)
	assert.NoError(t, err)
	assert.True(t, t1.IsZero())

	// 2. time.Time value
	now := time.Now()
	t2, err := parseBatchJobTime(now)
	assert.NoError(t, err)
	assert.Equal(t, now, t2)

	// 3. valid RFC3339 string
	rfcStr := "2026-06-04T12:00:00Z"
	t3, err := parseBatchJobTime(rfcStr)
	assert.NoError(t, err)
	assert.Equal(t, int64(2026), int64(t3.Year()))

	// 4. invalid string
	_, err = parseBatchJobTime("not-a-date")
	assert.Error(t, err)

	// 5. unsupported type
	_, err = parseBatchJobTime(12345)
	assert.Error(t, err)
}

// TestGetActiveBatchJobs_DateCorruption verifies that if there's a corrupt/invalid
// createdAt or updatedAt type/format in the DB, GetActiveBatchJobs logs a warning and falls back to a zero time.
func TestGetActiveBatchJobs_DateCorruption(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	ctx := context.Background()

	// Ensure cleanup
	defer func() {
		cleanupJobs(p)
	}()
	cleanupJobs(p)

	// Create a job node with a corrupt createdAt property (as an integer)
	corruptJobID := "test-job-corrupt-date"
	setupQuery := `
		CREATE (j:BatchJob {
			jobID: $jobID,
			modelName: 'gemini-embedding-001',
			gcsInputURI: 'gs://in',
			gcsOutputURI: 'gs://out',
			state: 'pending',
			createdAt: 12345, // corrupt: should be time or RFC3339 string
			updatedAt: $now
		})
	`
	_, err := neo4j.ExecuteQuery(ctx, p.driver, setupQuery, map[string]any{
		"jobID": corruptJobID,
		"now":   time.Now(),
	}, neo4j.EagerResultTransformer)
	assert.NoError(t, err)

	// Attempt to retrieve active jobs. It should succeed now by falling back to zero time!
	activeJobs, err := p.GetActiveBatchJobs(ctx)
	assert.NoError(t, err)
	
	found := false
	for _, job := range activeJobs {
		if job.JobID == corruptJobID {
			found = true
			assert.True(t, job.CreatedAt.IsZero(), "Expected CreatedAt to fall back to zero time")
			assert.False(t, job.UpdatedAt.IsZero(), "Expected UpdatedAt to be parsed successfully")
		}
	}
	assert.True(t, found, "Expected to retrieve the job even with a corrupt createdAt date")
}
