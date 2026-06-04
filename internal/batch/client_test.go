package batch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

// MockGenAIBatches implements GenAIBatches for unit testing, keeping in-memory state.
type MockGenAIBatches struct {
	mu   sync.RWMutex
	Jobs map[string]*genai.BatchJob
}

func NewMockGenAIBatches() *MockGenAIBatches {
	return &MockGenAIBatches{
		Jobs: make(map[string]*genai.BatchJob),
	}
}

func (m *MockGenAIBatches) SetJob(job *genai.BatchJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Jobs[job.Name] = job
}

func (m *MockGenAIBatches) Create(ctx context.Context, model string, src *genai.BatchJobSource, config *genai.CreateBatchJobConfig) (*genai.BatchJob, error) {
	if src == nil || len(src.GCSURI) == 0 {
		return nil, errors.New("invalid input config")
	}
	if config == nil || config.Dest == nil || config.Dest.GCSURI == "" {
		return nil, errors.New("invalid output config")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	jobID := fmt.Sprintf("projects/test-project/locations/us-central1/batchPredictionJobs/job-%d", len(m.Jobs)+1)
	job := &genai.BatchJob{
		Name:       jobID,
		State:      "JOB_STATE_PENDING",
		CreateTime: time.Now(),
	}
	m.Jobs[jobID] = job
	return job, nil
}

func (m *MockGenAIBatches) Get(ctx context.Context, name string, config *genai.GetBatchJobConfig) (*genai.BatchJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.Jobs[name]
	if !ok {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func TestRealBatchClient_CreateJob(t *testing.T) {
	ctx := context.Background()
	mockBatches := NewMockGenAIBatches()
	client := NewRealBatchClient(mockBatches)

	model := "gemini-1.5-flash"
	inputURI := "gs://my-bucket/inputs/req.jsonl"
	outputURI := "gs://my-bucket/outputs/"

	// Create a new job
	jobName, err := client.CreateJob(ctx, model, inputURI, outputURI)
	assert.NoError(t, err)
	assert.NotEmpty(t, jobName)
	assert.Contains(t, jobName, "batchPredictionJobs/job-1")

	// Verify job is recorded in the mocked server state
	storedJob, err := mockBatches.Get(ctx, jobName, nil)
	assert.NoError(t, err)
	assert.Equal(t, genai.JobState("JOB_STATE_PENDING"), storedJob.State)
}

func TestRealBatchClient_GetJobStatus(t *testing.T) {
	ctx := context.Background()
	mockBatches := NewMockGenAIBatches()
	client := NewRealBatchClient(mockBatches)

	// Pre-populate some states
	job1 := "projects/test-project/locations/us-central1/batchPredictionJobs/job-success"
	mockBatches.SetJob(&genai.BatchJob{
		Name:  job1,
		State: "JOB_STATE_SUCCEEDED",
	})

	job2 := "projects/test-project/locations/us-central1/batchPredictionJobs/job-failed"
	mockBatches.SetJob(&genai.BatchJob{
		Name:  job2,
		State: "JOB_STATE_FAILED",
		Error: &genai.JobError{
			Message: "internal processing error",
		},
	})

	// Test success status check
	name, state, failureReason, err := client.GetJobStatus(ctx, job1)
	assert.NoError(t, err)
	assert.Equal(t, job1, name)
	assert.Equal(t, "JOB_STATE_SUCCEEDED", state)
	assert.Empty(t, failureReason)

	// Test failed status check
	name2, state2, failureReason2, err := client.GetJobStatus(ctx, job2)
	assert.NoError(t, err)
	assert.Equal(t, job2, name2)
	assert.Equal(t, "JOB_STATE_FAILED", state2)
	assert.Equal(t, "internal processing error", failureReason2)

	// Test checking status of non-existent job
	_, _, _, err = client.GetJobStatus(ctx, "non-existent-job")
	assert.Error(t, err)
}

// ErrorGenAIBatches implements GenAIBatches and returns pre-configured errors and jobs.
type ErrorGenAIBatches struct {
	CreateErr error
	GetErr    error
	JobState  string
	JobError  *genai.JobError
	NilJob    bool
}

func (m *ErrorGenAIBatches) Create(ctx context.Context, model string, src *genai.BatchJobSource, config *genai.CreateBatchJobConfig) (*genai.BatchJob, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	if m.NilJob {
		return nil, nil
	}
	return &genai.BatchJob{
		Name:  "projects/test-project/locations/us-central1/batchPredictionJobs/error-job",
		State: genai.JobState(m.JobState),
		Error: m.JobError,
	}, nil
}

func (m *ErrorGenAIBatches) Get(ctx context.Context, name string, config *genai.GetBatchJobConfig) (*genai.BatchJob, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.NilJob {
		return nil, nil
	}
	return &genai.BatchJob{
		Name:  name,
		State: genai.JobState(m.JobState),
		Error: m.JobError,
	}, nil
}

func TestRealBatchClient_VertexAIErrorResponses(t *testing.T) {
	ctx := context.Background()

	// Test 1: CreateJob fails when Vertex AI Create returns an error
	{
		mockErr := errors.New("Vertex AI quota exceeded")
		mockBatches := &ErrorGenAIBatches{CreateErr: mockErr}
		client := NewRealBatchClient(mockBatches)

		_, err := client.CreateJob(ctx, "gemini-1.5-flash", "gs://in/req.jsonl", "gs://out/")
		assert.ErrorIs(t, err, mockErr)
		assert.Contains(t, err.Error(), "failed to create batch job")
	}

	// Test 2: CreateJob fails when Vertex AI Create returns nil job
	{
		mockBatches := &ErrorGenAIBatches{NilJob: true}
		client := NewRealBatchClient(mockBatches)

		_, err := client.CreateJob(ctx, "gemini-1.5-flash", "gs://in/req.jsonl", "gs://out/")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "received nil job from batch creation")
	}

	// Test 3: GetJobStatus fails when Vertex AI Get returns an error
	{
		mockErr := errors.New("Vertex AI service unavailable")
		mockBatches := &ErrorGenAIBatches{GetErr: mockErr}
		client := NewRealBatchClient(mockBatches)

		_, _, _, err := client.GetJobStatus(ctx, "job-id")
		assert.ErrorIs(t, err, mockErr)
		assert.Contains(t, err.Error(), "failed to get batch job status")
	}

	// Test 4: GetJobStatus fails when Vertex AI Get returns nil job
	{
		mockBatches := &ErrorGenAIBatches{NilJob: true}
		client := NewRealBatchClient(mockBatches)

		_, _, _, err := client.GetJobStatus(ctx, "job-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "received nil job from batch status check")
	}

	// Test 5: GetJobStatus succeeds and extracts job error when job itself fails
	{
		jobErr := &genai.JobError{
			Message: "invalid input format in jsonl",
		}
		mockBatches := &ErrorGenAIBatches{
			JobState: "JOB_STATE_FAILED",
			JobError: jobErr,
		}
		client := NewRealBatchClient(mockBatches)

		name, state, failureReason, err := client.GetJobStatus(ctx, "job-id")
		assert.NoError(t, err)
		assert.Equal(t, "job-id", name)
		assert.Equal(t, "JOB_STATE_FAILED", state)
		assert.Equal(t, "invalid input format in jsonl", failureReason)
	}
}

