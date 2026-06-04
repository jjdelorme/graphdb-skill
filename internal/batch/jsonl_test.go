package batch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRequestsJSONL(t *testing.T) {
	items := []RequestItem{
		{
			CustomID: "func_123",
			Prompt:   "Explain this code: func main() {}",
		},
		{
			CustomID: "func_456",
			Prompt:   "Optimize this loop: for i := range n {}",
		},
	}

	jsonl, err := GenerateRequestsJSONL(items)
	assert.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(jsonl), "\n")
	assert.Len(t, lines, 2)

	// Validate first request line
	var req1 BatchRequest
	err = json.Unmarshal([]byte(lines[0]), &req1)
	assert.NoError(t, err)
	assert.Equal(t, "func_123", req1.CustomID)
	assert.NotNil(t, req1.Request)
	assert.Len(t, req1.Request.Contents, 1)
	assert.Equal(t, "user", req1.Request.Contents[0].Role)
	assert.Len(t, req1.Request.Contents[0].Parts, 1)
	assert.Equal(t, "Explain this code: func main() {}", req1.Request.Contents[0].Parts[0].Text)

	// Validate second request line
	var req2 BatchRequest
	err = json.Unmarshal([]byte(lines[1]), &req2)
	assert.NoError(t, err)
	assert.Equal(t, "func_456", req2.CustomID)
	assert.NotNil(t, req2.Request)
	assert.Len(t, req2.Request.Contents, 1)
	assert.Equal(t, "user", req2.Request.Contents[0].Role)
	assert.Len(t, req2.Request.Contents[0].Parts, 1)
	assert.Equal(t, "Optimize this loop: for i := range n {}", req2.Request.Contents[0].Parts[0].Text)
}

func TestParseResponsesJSONL(t *testing.T) {
	// A valid JSONL response from Vertex AI Batch API
	responseJSONL := `{"customId": "func_123", "response": {"candidates": [{"content": {"parts": [{"text": "Summarized function description"}]}}]}}
{"customId": "func_456", "error": {"code": 400, "message": "Resource exhausted"}}
`

	results, err := ParseResponsesJSONL(responseJSONL)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify first item (success)
	assert.Equal(t, "func_123", results[0].CustomID)
	assert.Equal(t, "Summarized function description", results[0].Text)
	assert.Empty(t, results[0].Error)

	// Verify second item (error)
	assert.Equal(t, "func_456", results[1].CustomID)
	assert.Empty(t, results[1].Text)
	assert.Equal(t, "API error code 400: Resource exhausted", results[1].Error)
}

func TestParseResponsesJSONL_InvalidJSON(t *testing.T) {
	invalidJSONL := `{"customId": "func_123", "response": {"candidates" :`
	results, err := ParseResponsesJSONL(invalidJSONL)
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestGenerateRequestsJSONL_EmptyList(t *testing.T) {
	// Test nil
	jsonl, err := GenerateRequestsJSONL(nil)
	assert.NoError(t, err)
	assert.Equal(t, "", jsonl)

	// Test empty slice
	jsonl2, err := GenerateRequestsJSONL([]RequestItem{})
	assert.NoError(t, err)
	assert.Equal(t, "", jsonl2)
}

func TestGenerateRequestsJSONL_LargePayload(t *testing.T) {
	// Generate a very large prompt (10 MB)
	largePrompt := strings.Repeat("A", 10*1024*1024)
	items := []RequestItem{
		{
			CustomID: "func_large",
			Prompt:   largePrompt,
		},
	}
	jsonl, err := GenerateRequestsJSONL(items)
	assert.NoError(t, err)

	// Verify we can parse it back
	var req BatchRequest
	err = json.Unmarshal([]byte(jsonl), &req)
	assert.NoError(t, err)
	assert.Equal(t, "func_large", req.CustomID)
	assert.Len(t, req.Request.Contents, 1)
	assert.Equal(t, largePrompt, req.Request.Contents[0].Parts[0].Text)
}

func TestParseResponsesJSONL_LargeResponse(t *testing.T) {
	// Generate a response candidate text larger than the default 64KB bufio.Scanner limit (e.g., 200KB)
	largeText := strings.Repeat("B", 200*1024)
	responseJSONL := `{"customId": "func_large_resp", "response": {"candidates": [{"content": {"parts": [{"text": "` + largeText + `"}]}}]}}`

	results, err := ParseResponsesJSONL(responseJSONL)
	
	// We assert that the function parsed it successfully. 
	// If bufio.Scanner fails due to token size being too large, this assertion will fail, exposing the bug.
	if assert.NoError(t, err) {
		assert.Len(t, results, 1)
		assert.Equal(t, "func_large_resp", results[0].CustomID)
		assert.Equal(t, largeText, results[0].Text)
	}
}

