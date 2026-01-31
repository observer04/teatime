package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Storage handles Cloudflare R2 operations using AWS SDK v2
type R2Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewR2Storage creates a new R2 storage client
func NewR2Storage(accountID, accessKeyID, secretAccessKey, bucket string) (*R2Storage, error) {
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucket == "" {
		return nil, fmt.Errorf("R2 configuration incomplete")
	}

	// R2 endpoint format
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	// Create AWS credentials
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	// Create S3 client configured for R2
	client := s3.New(s3.Options{
		Region:       "auto",
		Credentials:  creds,
		BaseEndpoint: aws.String(endpoint),
	})

	// Create presigner
	presigner := s3.NewPresignClient(client)

	return &R2Storage{
		client:    client,
		presigner: presigner,
		bucket:    bucket,
	}, nil
}

// GeneratePresignedPutURL generates a presigned URL for uploading a file
func (r *R2Storage) GeneratePresignedPutURL(ctx context.Context, objectKey string, contentType string, expiryDuration time.Duration) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}

	request, err := r.presigner.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiryDuration
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT URL: %w", err)
	}

	return request.URL, nil
}

// GeneratePresignedGetURL generates a presigned URL for downloading a file
func (r *R2Storage) GeneratePresignedGetURL(ctx context.Context, objectKey string, expiryDuration time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(objectKey),
	}

	request, err := r.presigner.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiryDuration
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	return request.URL, nil
}

// DeleteObject deletes an object from R2
func (r *R2Storage) DeleteObject(ctx context.Context, objectKey string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(objectKey),
	}

	_, err := r.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}
