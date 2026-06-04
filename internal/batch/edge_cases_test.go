package batch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

// MockErrorGenAIBatches is a mock batches client that allows simulating errors.
type MockErrorGenAIBatches struct {
	CreateErr   error
	CreateVal   *genai.BatchJob
	GetErr      error
	GetVal      *genai.BatchJob
	CreateCalls int
	GetCalls    int
}

func (m *MockErrorGenAIBatches) Create(ctx context.Context, model string, src *genai.BatchJobSource, config *genai.CreateBatchJobConfig) (*genai.BatchJob, error) {
	m.CreateCalls++
	return m.CreateVal, m.CreateErr
}

func (m *MockErrorGenAIBatches) Get(ctx context.Context, name string, config *genai.GetBatchJobConfig) (*genai.BatchJob, error) {
	m.GetCalls++
	return m.GetVal, m.GetErr
}

// TestGenerateRequestsJSONL_EdgeCases tests edge cases for GenerateRequestsJSONL.
func TestGenerateRequestsJSONL_EdgeCases(t *testing.T) {
	// 1. Empty and nil slices
	resNil, err := GenerateRequestsJSONL(nil)
	assert.NoError(t, err)
	assert.Empty(t, resNil)

	resEmpty, err := GenerateRequestsJSONL([]RequestItem{})
	assert.NoError(t, err)
	assert.Empty(t, resEmpty)

	// 2. Large prompt payload (100KB)
	largePrompt := strings.Repeat("A", 100000)
	items := []RequestItem{
		{
			CustomID: "func_large",
			Prompt:   largePrompt,
		},
	}
	resLarge, err := GenerateRequestsJSONL(items)
	assert.NoError(t, err)
	assert.NotEmpty(t, resLarge)
	assert.Contains(t, resLarge, "func_large")
	assert.Contains(t, resLarge, largePrompt)
}

// TestParseResponsesJSONL_EdgeCases tests edge cases for ParseResponsesJSONL.
func TestParseResponsesJSONL_EdgeCases(t *testing.T) {
	// 1. Empty input
	resNil, err := ParseResponsesJSONL("")
	assert.NoError(t, err)
	assert.Empty(t, resNil)

	resWhitespace, err := ParseResponsesJSONL("  \n  \n")
	assert.NoError(t, err)
	assert.Empty(t, resWhitespace)

	// 2. Large response payload (> 64KB, e.g. 100KB)
	// We expect this to succeed now due to the custom 10MB scanner buffer limit.
	largeText := strings.Repeat("B", 100000)
	largeJSON := fmt.Sprintf(`{"customId": "func_large", "response": {"candidates": [{"content": {"parts": [{"text": "%s"}]}}]}}`+"\n", largeText)
	resLarge, errLarge := ParseResponsesJSONL(largeJSON)
	assert.NoError(t, errLarge)
	assert.Len(t, resLarge, 1)
	assert.Equal(t, "func_large", resLarge[0].CustomID)
	assert.Equal(t, largeText, resLarge[0].Text)

	// 3. API Error handling
	apiErrorJSONL := `{"customId": "func_err", "error": {"code": 500, "message": "internal server error"}}`
	resAPIError, err := ParseResponsesJSONL(apiErrorJSONL)
	assert.NoError(t, err)
	assert.Len(t, resAPIError, 1)
	assert.Equal(t, "func_err", resAPIError[0].CustomID)
	assert.Equal(t, "API error code 500: internal server error", resAPIError[0].Error)

	// 4. Malformed JSON line
	malformedJSONL := `{"customId": "func_1", "response": {`
	resMalformed, errMalformed := ParseResponsesJSONL(malformedJSONL)
	assert.Error(t, errMalformed)
	assert.Nil(t, resMalformed)

	// 5. Empty candidates
	emptyCandidatesJSONL := `{"customId": "func_empty_cand", "response": {"candidates": []}}`
	resEmptyCand, err := ParseResponsesJSONL(emptyCandidatesJSONL)
	assert.NoError(t, err)
	assert.Len(t, resEmptyCand, 1)
	assert.Equal(t, "func_empty_cand", resEmptyCand[0].CustomID)
	assert.Empty(t, resEmptyCand[0].Text)

	// 6. Nil content inside candidate
	nilContentJSONL := `{"customId": "func_nil_content", "response": {"candidates": [{"content": null}]}}`
	resNilContent, err := ParseResponsesJSONL(nilContentJSONL)
	assert.NoError(t, err)
	assert.Len(t, resNilContent, 1)
	assert.Equal(t, "func_nil_content", resNilContent[0].CustomID)
	assert.Empty(t, resNilContent[0].Text)

	// 7. Empty parts in content
	emptyPartsJSONL := `{"customId": "func_empty_parts", "response": {"candidates": [{"content": {"parts": []}}]}}`
	resEmptyParts, err := ParseResponsesJSONL(emptyPartsJSONL)
	assert.NoError(t, err)
	assert.Len(t, resEmptyParts, 1)
	assert.Equal(t, "func_empty_parts", resEmptyParts[0].CustomID)
	assert.Empty(t, resEmptyParts[0].Text)

	// 8. Multiple parts (first one should be selected)
	multiplePartsJSONL := `{"customId": "func_mult_parts", "response": {"candidates": [{"content": {"parts": [{"text": "first_part"}, {"text": "second_part"}]}}]}}`
	resMultParts, err := ParseResponsesJSONL(multiplePartsJSONL)
	assert.NoError(t, err)
	assert.Len(t, resMultParts, 1)
	assert.Equal(t, "func_mult_parts", resMultParts[0].CustomID)
	assert.Equal(t, "first_part", resMultParts[0].Text)
}

// TestRealBatchClient_CreateJob_EdgeCases tests RealBatchClient.CreateJob edge cases.
func TestRealBatchClient_CreateJob_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// 1. Vertex AI returns an error during creation
	mock := &MockErrorGenAIBatches{
		CreateErr: errors.New("quota exceeded"),
	}
	client := NewRealBatchClient(mock)
	_, err := client.CreateJob(ctx, "gemini-1.5-flash", "gs://in", "gs://out")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")

	// 2. Vertex AI returns nil job and nil error during creation
	mockNil := &MockErrorGenAIBatches{
		CreateVal: nil,
		CreateErr: nil,
	}
	clientNil := NewRealBatchClient(mockNil)
	_, errNil := clientNil.CreateJob(ctx, "gemini-1.5-flash", "gs://in", "gs://out")
	assert.Error(t, errNil)
	assert.Contains(t, errNil.Error(), "received nil job")
}

// TestRealBatchClient_GetJobStatus_EdgeCases tests RealBatchClient.GetJobStatus edge cases.
func TestRealBatchClient_GetJobStatus_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// 1. Vertex AI returns an error during status retrieve
	mock := &MockErrorGenAIBatches{
		GetErr: errors.New("permission denied"),
	}
	client := NewRealBatchClient(mock)
	_, _, _, err := client.GetJobStatus(ctx, "job_id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")

	// 2. Vertex AI returns nil job and nil error during status retrieve
	mockNil := &MockErrorGenAIBatches{
		GetVal: nil,
		GetErr: nil,
	}
	clientNil := NewRealBatchClient(mockNil)
	_, _, _, errNil := clientNil.GetJobStatus(ctx, "job_id")
	assert.Error(t, errNil)
	assert.Contains(t, errNil.Error(), "received nil job")

	// 3. Job fails but has no error structure (error field is nil in BatchJob)
	mockFailedNoErr := &MockErrorGenAIBatches{
		GetVal: &genai.BatchJob{
			Name:  "job_failed_no_err",
			State: "JOB_STATE_FAILED",
			Error: nil,
		},
	}
	clientFailedNoErr := NewRealBatchClient(mockFailedNoErr)
	name, state, reason, err := clientFailedNoErr.GetJobStatus(ctx, "job_failed_no_err")
	assert.NoError(t, err)
	assert.Equal(t, "job_failed_no_err", name)
	assert.Equal(t, "JOB_STATE_FAILED", state)
	assert.Empty(t, reason)
}
