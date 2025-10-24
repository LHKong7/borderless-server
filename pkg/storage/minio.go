package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"borderless_coding_server/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinIOClient *minio.Client

func ConnectMinIO(cfg *config.Config) error {
	var err error
	MinIOClient, err = minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})

	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = MinIOClient.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to MinIO: %w", err)
	}

	// Create bucket if it doesn't exist
	err = createBucketIfNotExists(ctx, cfg.MinIOBucketName)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	log.Println("MinIO connected successfully")
	return nil
}

func createBucketIfNotExists(ctx context.Context, bucketName string) error {
	exists, err := MinIOClient.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}

	if !exists {
		err = MinIOClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
		log.Printf("Created bucket: %s", bucketName)
	}

	return nil
}

// UploadFile uploads a file to MinIO
func UploadFile(ctx context.Context, bucketName, objectName, filePath string) error {
	_, err := MinIOClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{})
	return err
}

// DownloadFile downloads a file from MinIO
func DownloadFile(ctx context.Context, bucketName, objectName, filePath string) error {
	return MinIOClient.FGetObject(ctx, bucketName, objectName, filePath, minio.GetObjectOptions{})
}

// GetPresignedURL generates a presigned URL for an object
func GetPresignedURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	url, err := MinIOClient.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// DeleteObject deletes an object from MinIO
func DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return MinIOClient.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
}

// ListObjects lists objects in a bucket
func ListObjects(ctx context.Context, bucketName string, prefix string) <-chan minio.ObjectInfo {
	return MinIOClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
}
