package batch

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// GCSClient defines the interface for Google Cloud Storage operations.
type GCSClient interface {
	Upload(ctx context.Context, bucket, objectPath, data string) error
	Download(ctx context.Context, bucket, objectPath string) (string, error)
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

// RealGCSClient implements GCSClient using the official GCP storage package.
type RealGCSClient struct {
	client *storage.Client
}

// NewRealGCSClient creates a new GCS client wrapper.
func NewRealGCSClient(client *storage.Client) *RealGCSClient {
	return &RealGCSClient{client: client}
}

// Upload uploads the string data to the specified GCS bucket and object path.
func (c *RealGCSClient) Upload(ctx context.Context, bucket, objectPath, data string) error {
	wc := c.client.Bucket(bucket).Object(objectPath).NewWriter(ctx)
	if _, err := io.WriteString(wc, data); err != nil {
		_ = wc.Close()
		return err
	}
	return wc.Close()
}

// Download downloads the object content from GCS as a string.
func (c *RealGCSClient) Download(ctx context.Context, bucket, objectPath string) (string, error) {
	rc, err := c.client.Bucket(bucket).Object(objectPath).NewReader(ctx)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ListObjects lists the object names matching the given prefix in the GCS bucket.
func (c *RealGCSClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	var names []string
	it := c.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		names = append(names, attrs.Name)
	}
	return names, nil
}
