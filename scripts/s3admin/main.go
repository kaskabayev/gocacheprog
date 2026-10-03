// s3admin is a small helper for the manual integration test.
//
// Usage: go run ./scripts/s3admin
//
// Creates the S3 bucket used by scripts/manual-e2e.sh if it does not exist.
// Configuration comes from S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and
// S3_SECRET_KEY, with the same defaults as the test script.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(envOr("S3_ENDPOINT", "http://127.0.0.1:8333"), "https://"), "http://")
	bucket := envOr("S3_BUCKET", "gobuilds")

	client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			envOr("S3_ACCESS_KEY", "gocachekey"),
			envOr("S3_SECRET_KEY", "gocachesecret"), ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		log.Fatalf("cannot reach S3 at %s: %v", endpoint, err)
	}
	if exists {
		log.Printf("bucket %s already exists", bucket)
		return
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		log.Fatal(err)
	}
	log.Printf("bucket %s created", bucket)
}
