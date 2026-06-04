package batch

import (
	"context"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
)

// MockGCSClient is a mock GCSClient for testing that maintains real in-memory state.
type MockGCSClient struct {
	mu      sync.RWMutex
	// Storage maps bucket -> objectPath -> content
	Storage map[string]map[string]string
}

func NewMockGCSClient() *MockGCSClient {
	return &MockGCSClient{
		Storage: make(map[string]map[string]string),
	}
}

func (m *MockGCSClient) Upload(ctx context.Context, bucket, objectPath, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Storage[bucket] == nil {
		m.Storage[bucket] = make(map[string]string)
	}
	m.Storage[bucket][objectPath] = data
	return nil
}

func (m *MockGCSClient) Download(ctx context.Context, bucket, objectPath string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucketData, ok := m.Storage[bucket]
	if !ok {
		return "", storage.ErrBucketNotExist // Bucket not found
	}
	data, ok := bucketData[objectPath]
	if !ok {
		return "", storage.ErrObjectNotExist // Object not found
	}
	return data, nil
}

func (m *MockGCSClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucketData, ok := m.Storage[bucket]
	if !ok {
		return nil, storage.ErrBucketNotExist
	}
	var results []string
	for path := range bucketData {
		if strings.HasPrefix(path, prefix) {
			results = append(results, path)
		}
	}
	return results, nil
}

func TestMockGCSClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockGCSClient()

	bucket := "my-bucket"
	path1 := "input/request1.jsonl"
	data1 := `{"request": {"contents": [{"role": "user", "parts": [{"text": "hello"}]}]}}`

	path2 := "input/request2.jsonl"
	data2 := `{"request": {"contents": [{"role": "user", "parts": [{"text": "world"}]}]}}`

	// Test Upload
	err := client.Upload(ctx, bucket, path1, data1)
	assert.NoError(t, err)

	err = client.Upload(ctx, bucket, path2, data2)
	assert.NoError(t, err)

	// Test Download
	downloaded1, err := client.Download(ctx, bucket, path1)
	assert.NoError(t, err)
	assert.Equal(t, data1, downloaded1)

	// Test Download non-existent object returns standard error
	_, err = client.Download(ctx, bucket, "input/nonexistent.jsonl")
	assert.ErrorIs(t, err, storage.ErrObjectNotExist)

	// Test Download non-existent bucket returns standard error
	_, err = client.Download(ctx, "nonexistent-bucket", path1)
	assert.ErrorIs(t, err, storage.ErrBucketNotExist)

	// Test ListObjects
	list, err := client.ListObjects(ctx, bucket, "input/")
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Contains(t, list, path1)
	assert.Contains(t, list, path2)

	// Test ListObjects with non-existent bucket returns standard error
	_, err = client.ListObjects(ctx, "nonexistent-bucket", "input/")
	assert.ErrorIs(t, err, storage.ErrBucketNotExist)

	// Test ListObjects with prefix mismatch
	listEmpty, err := client.ListObjects(ctx, bucket, "output/")
	assert.NoError(t, err)
	assert.Empty(t, listEmpty)
}

// ErrorMockGCSClient is a mock GCSClient that returns pre-configured errors.
type ErrorMockGCSClient struct {
	UploadErr   error
	DownloadErr error
	ListErr     error
}

func (m *ErrorMockGCSClient) Upload(ctx context.Context, bucket, objectPath, data string) error {
	return m.UploadErr
}

func (m *ErrorMockGCSClient) Download(ctx context.Context, bucket, objectPath string) (string, error) {
	return "", m.DownloadErr
}

func (m *ErrorMockGCSClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	return nil, m.ListErr
}

func TestErrorMockGCSClient(t *testing.T) {
	ctx := context.Background()
	expectedErr := assert.AnError
	client := &ErrorMockGCSClient{
		UploadErr:   expectedErr,
		DownloadErr: expectedErr,
		ListErr:     expectedErr,
	}

	// Verify Upload returns error
	err := client.Upload(ctx, "my-bucket", "path", "data")
	assert.ErrorIs(t, err, expectedErr)

	// Verify Download returns error
	_, err = client.Download(ctx, "my-bucket", "path")
	assert.ErrorIs(t, err, expectedErr)

	// Verify ListObjects returns error
	_, err = client.ListObjects(ctx, "my-bucket", "prefix")
	assert.ErrorIs(t, err, expectedErr)
}

