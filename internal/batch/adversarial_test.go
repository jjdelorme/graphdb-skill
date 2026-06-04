package batch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
)

// TestParseResponsesJSONL_Panics tests crash/panic resilience of ParseResponsesJSONL
// when encountering invalid structure configurations (nil candidate objects/contents/parts).
func TestParseResponsesJSONL_Panics(t *testing.T) {
	// 1. Nil Candidate: candidates contains a null element
	nilCandJSON := `{"customId": "func_nil_cand", "response": {"candidates": [null]}}`
	
	// We assert that this does not panic because of the cand != nil check.
	assert.NotPanics(t, func() {
		res, err := ParseResponsesJSONL(nilCandJSON)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Empty(t, res[0].Text)
	})

	// 2. Nil Candidate Content
	nilContentJSON := `{"customId": "func_nil_content", "response": {"candidates": [{"content": null}]}}`
	// This should not panic because cand.Content != nil check exists.
	assert.NotPanics(t, func() {
		res, err := ParseResponsesJSONL(nilContentJSON)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Empty(t, res[0].Text)
	})

	// 3. Nil Candidate Parts
	nilPartsJSON := `{"customId": "func_nil_parts", "response": {"candidates": [{"content": {"parts": [null]}}]}}`
	// This should not panic because cand.Content.Parts[0] != nil check exists.
	assert.NotPanics(t, func() {
		res, err := ParseResponsesJSONL(nilPartsJSON)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Empty(t, res[0].Text)
	})
}

// TestParseResponsesJSONL_ScannerTooLong verifies behavior when a single line
// in the JSONL payload exceeds the maximum 10MB scanner buffer limit.
func TestParseResponsesJSONL_ScannerTooLong(t *testing.T) {
	// Generate an 11MB string
	longText := strings.Repeat("X", 11*1024*1024)
	largeJSON := fmt.Sprintf(`{"customId": "func_too_long", "response": {"candidates": [{"content": {"parts": [{"text": "%s"}]}}]}}`+"\n", longText)

	res, err := ParseResponsesJSONL(largeJSON)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "error reading response JSONL")
}

// TestParseResponsesJSONL_AllCorruptJSON verifies that when all JSON lines
// are malformed, it returns an error instead of succeeding silently.
func TestParseResponsesJSONL_AllCorruptJSON(t *testing.T) {
	corruptInput := "invalid json line 1\n{invalid json line 2}\n"
	res, err := ParseResponsesJSONL(corruptInput)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to parse any valid lines")
}

// TestRealBatchClient_Concurrency stress-tests concurrent calls to RealBatchClient operations.
func TestRealBatchClient_Concurrency(t *testing.T) {
	ctx := context.Background()
	mockBatches := NewMockGenAIBatches()
	client := NewRealBatchClient(mockBatches)

	var wg sync.WaitGroup
	numWorkers := 20
	jobsPerWorker := 10

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < jobsPerWorker; i++ {
				model := fmt.Sprintf("model-%d-%d", workerID, i)
				input := fmt.Sprintf("gs://bucket/input-%d-%d", workerID, i)
				output := fmt.Sprintf("gs://bucket/output-%d-%d", workerID, i)

				// Create job
				jobName, err := client.CreateJob(ctx, model, input, output)
				if assert.NoError(t, err) {
					assert.NotEmpty(t, jobName)

					// Check status immediately
					name, state, _, err := client.GetJobStatus(ctx, jobName)
					assert.NoError(t, err)
					assert.Equal(t, jobName, name)
					assert.Equal(t, "JOB_STATE_PENDING", state)
				}
			}
		}(w)
	}

	wg.Wait()
}

// TestRealGCSClient_Errors tests RealGCSClient error behavior when configured with
// unauthenticated clients or cancelled context.
func TestRealGCSClient_Errors(t *testing.T) {
	ctx := context.Background()

	// Initialize storage.Client without authentication so it's a real client struct
	// but cannot make actual cloud calls (or will fail quickly).
	gcsStorageClient, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Skip("Skipping RealGCSClient unit test as storage client couldn't be initialized")
		return
	}
	defer gcsStorageClient.Close()

	gcsClient := NewRealGCSClient(gcsStorageClient)

	// 1. Cancelled Context Upload
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // cancel context immediately

	err = gcsClient.Upload(cancelCtx, "test-bucket", "path", "data")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "canceled"))

	// 2. Cancelled Context Download
	_, err = gcsClient.Download(cancelCtx, "test-bucket", "path")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "canceled"))

	// 3. Cancelled Context ListObjects
	_, err = gcsClient.ListObjects(cancelCtx, "test-bucket", "prefix")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "canceled"))
}
