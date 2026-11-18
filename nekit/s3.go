package nekit

import (
	"github.com/minio/minio-go/v7"
	"neui.dev/nekit/internal/s3"
)

// S3Uploads keeps uploaded files in an S3-compatible bucket. Plug it in with
// neui.WithUploadStorage or neui.OptionUploadStorage. The service must support
// conditional writes (If-None-Match), as AWS S3 and MinIO do.
type S3Uploads = *s3.Store

// NewS3Uploads stores files in bucket under prefix, "" for the bucket root. Build
// client with minio.New for your endpoint and credentials; the bucket must
// exist. Each upload buffers up to 16 MiB in memory.
func NewS3Uploads(client *minio.Client, bucket, prefix string) S3Uploads {
	return s3.New(client, bucket, prefix)
}
