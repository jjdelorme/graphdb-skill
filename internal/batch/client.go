package batch

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// BatchClient defines a mockable interface for managing Vertex AI Batch Prediction jobs.
type BatchClient interface {
	CreateJob(ctx context.Context, model string, gcsInputURI, gcsOutputURI string) (string, error)
	GetJobStatus(ctx context.Context, jobNameID string) (string, string, string, error) // Returns (jobID, state, failureReason, error)
}

// GenAIBatches defines the interface for the underlying GenAI batches service methods we use.
type GenAIBatches interface {
	Create(ctx context.Context, model string, src *genai.BatchJobSource, config *genai.CreateBatchJobConfig) (*genai.BatchJob, error)
	Get(ctx context.Context, name string, config *genai.GetBatchJobConfig) (*genai.BatchJob, error)
}

// RealBatchClient implements BatchClient by wrapping the GenAI SDK's Batches service.
type RealBatchClient struct {
	batches GenAIBatches
}

// NewRealBatchClient creates a new RealBatchClient wrapping the given GenAIBatches service.
func NewRealBatchClient(batches GenAIBatches) *RealBatchClient {
	return &RealBatchClient{
		batches: batches,
	}
}

// CreateJob submits a new batch prediction job using Vertex AI.
func (c *RealBatchClient) CreateJob(ctx context.Context, model string, gcsInputURI, gcsOutputURI string) (string, error) {
	src := &genai.BatchJobSource{
		Format: "jsonl",
		GCSURI: []string{gcsInputURI},
	}
	displayName := "graphdb-enrich"
	parts := strings.Split(gcsInputURI, "/")
	if len(parts) >= 2 {
		jobID := parts[len(parts)-2]
		if jobID != "" && jobID != "graphdb-batches" {
			displayName = fmt.Sprintf("graphdb-enrich-%s", jobID)
		}
	}

	config := &genai.CreateBatchJobConfig{
		DisplayName: displayName,
		Dest: &genai.BatchJobDestination{
			Format: "jsonl",
			GCSURI: gcsOutputURI,
		},
	}

	job, err := c.batches.Create(ctx, model, src, config)
	if err != nil {
		return "", fmt.Errorf("failed to create batch job: %w", err)
	}
	if job == nil {
		return "", fmt.Errorf("received nil job from batch creation")
	}
	return job.Name, nil
}

// GetJobStatus fetches the current state and details of an active batch prediction job.
func (c *RealBatchClient) GetJobStatus(ctx context.Context, jobNameID string) (string, string, string, error) {
	job, err := c.batches.Get(ctx, jobNameID, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get batch job status: %w", err)
	}
	if job == nil {
		return "", "", "", fmt.Errorf("received nil job from batch status check")
	}

	var failureReason string
	if job.Error != nil {
		failureReason = job.Error.Message
	}

	return job.Name, string(job.State), failureReason, nil
}
