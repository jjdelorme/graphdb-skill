package batch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

// TestParseResponsesJSONL_Adversarial tests corrupt JSONL payloads, null bytes,
// invalid structure, and lines exceeding the 10MB scanner buffer limit.
func TestParseResponsesJSONL_Adversarial(t *testing.T) {
	// Case 1: Corrupt line followed by valid line, followed by empty line, followed by valid line
	corruptPayload := `{"customId": "func_1", "response": {"candidates": [{"content": {"parts": [{"text": "valid_1"}]}}]}}
{"customId": "func_corrupt", "response": { INVALID JSON HERE }
  
{"customId": "func_2", "response": {"candidates": [{"content": {"parts": [{"text": "valid_2"}]}}]}}`

	results, err := ParseResponsesJSONL(corruptPayload)
	assert.NoError(t, err) // It logs warning and skips corrupt line
	assert.Len(t, results, 2)
	assert.Equal(t, "func_1", results[0].CustomID)
	assert.Equal(t, "valid_1", results[0].Text)
	assert.Equal(t, "func_2", results[1].CustomID)
	assert.Equal(t, "valid_2", results[1].Text)

	// Case 2: Injected null bytes in the JSON lines
	nullBytePayload := `{"customId": "func_null", "response": {"candidates": [{"content": {"parts": [{"text": "text` + string([]byte{0x00}) + `with_null"}]}}]}}`
	resultsNull, errNull := ParseResponsesJSONL(nullBytePayload)
	assert.NoError(t, errNull)
	assert.Len(t, resultsNull, 1)
	assert.Equal(t, "func_null", resultsNull[0].CustomID)
	assert.Contains(t, resultsNull[0].Text, "text")

	// Case 3: Line exceeding 10MB scanner limit (e.g. 11MB)
	// This should cause the scanner to fail and return an error (token too long)
	hugeText := strings.Repeat("C", 11*1024*1024)
	hugePayload := `{"customId": "func_huge", "response": {"candidates": [{"content": {"parts": [{"text": "` + hugeText + `"}]}}]}}`
	resultsHuge, errHuge := ParseResponsesJSONL(hugePayload)
	assert.Error(t, errHuge)
	assert.Contains(t, errHuge.Error(), "token too long")
	assert.Nil(t, resultsHuge)

	// Case 4: Totally random text (no newlines, invalid JSON)
	resultsRandom, errRandom := ParseResponsesJSONL("random content here without json format")
	assert.Error(t, errRandom)
	assert.Nil(t, resultsRandom)
	assert.Contains(t, errRandom.Error(), "failed to parse any valid lines")
}

// TestRealBatchClient_ConcurrencyAndRace verifies that the batch client can
// handle parallel queries to CreateJob and GetJobStatus safely.
func TestRealBatchClient_ConcurrencyAndRace(t *testing.T) {
	ctx := context.Background()
	mockBatches := NewMockGenAIBatches()
	client := NewRealBatchClient(mockBatches)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			inputURI := fmt.Sprintf("gs://bucket/inputs/req-%d.jsonl", idx)
			outputURI := fmt.Sprintf("gs://bucket/outputs/%d/", idx)

			// Submit job
			jobName, err := client.CreateJob(ctx, "gemini-1.5-flash", inputURI, outputURI)
			if !assert.NoError(t, err) {
				return
			}
			assert.NotEmpty(t, jobName)

			// Query status of the created job
			name, state, _, err := client.GetJobStatus(ctx, jobName)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, jobName, name)
			assert.Equal(t, "JOB_STATE_PENDING", state)
		}(i)
	}

	wg.Wait()
}

// TestRealBatchClient_VertexAIErrorMapping tests how RealBatchClient handles
// transient Vertex AI failures, quota limits (429), and bad gateway (502/503).
func TestRealBatchClient_VertexAIErrorMapping(t *testing.T) {
	ctx := context.Background()

	// 1. Quota Exceeded / Rate Limit (429) simulation
	mockRateLimit := &MockErrorGenAIBatches{
		CreateErr: errors.New("googleapi: Error 429: Resource exhausted"),
		GetErr:    errors.New("googleapi: Error 429: Resource exhausted"),
	}
	clientRateLimit := NewRealBatchClient(mockRateLimit)

	_, errCreate := clientRateLimit.CreateJob(ctx, "gemini-1.5-flash", "gs://in", "gs://out")
	assert.Error(t, errCreate)
	assert.Contains(t, errCreate.Error(), "429: Resource exhausted")

	_, _, _, errGet := clientRateLimit.GetJobStatus(ctx, "job_id")
	assert.Error(t, errGet)
	assert.Contains(t, errGet.Error(), "429: Resource exhausted")

	// 2. Vertex AI Service Outage (503) simulation
	mockOutage := &MockErrorGenAIBatches{
		CreateErr: errors.New("googleapi: Error 503: Service Unavailable"),
		GetErr:    errors.New("googleapi: Error 503: Service Unavailable"),
	}
	clientOutage := NewRealBatchClient(mockOutage)

	_, errCreateOut := clientOutage.CreateJob(ctx, "gemini-1.5-flash", "gs://in", "gs://out")
	assert.Error(t, errCreateOut)
	assert.Contains(t, errCreateOut.Error(), "503: Service Unavailable")

	_, _, _, errGetOut := clientOutage.GetJobStatus(ctx, "job_id")
	assert.Error(t, errGetOut)
	assert.Contains(t, errGetOut.Error(), "503: Service Unavailable")
}

// TestRealBatchClient_JobStatusCancellation verifies that GetJobStatus behaves
// correctly when the job returns active/pending states vs cancelled/expired states.
func TestRealBatchClient_JobStatusStates(t *testing.T) {
	ctx := context.Background()

	states := []string{
		"JOB_STATE_PENDING",
		"JOB_STATE_RUNNING",
		"JOB_STATE_CANCELLING",
		"JOB_STATE_CANCELLED",
		"JOB_STATE_PAUSED",
		"JOB_STATE_EXPIRED",
	}

	for _, s := range states {
		t.Run(s, func(t *testing.T) {
			mock := &MockErrorGenAIBatches{
				GetVal: &genai.BatchJob{
					Name:  "test-job",
					State: genai.JobState(s),
				},
			}
			client := NewRealBatchClient(mock)
			name, state, reason, err := client.GetJobStatus(ctx, "test-job")
			assert.NoError(t, err)
			assert.Equal(t, "test-job", name)
			assert.Equal(t, s, state)
			assert.Empty(t, reason)
		})
	}
}
