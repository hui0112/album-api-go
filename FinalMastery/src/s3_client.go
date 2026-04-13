package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Client handles S3 operations
type S3Client struct {
	client     *s3.Client
	bucketName string
	region     string
}

// NewS3Client creates a new S3 client
func NewS3Client(client *s3.Client, bucketName, region string) *S3Client {
	return &S3Client{
		client:     client,
		bucketName: bucketName,
		region:     region,
	}
}

// UploadFile uploads a file to S3 and returns the public URL
func (s *S3Client) UploadFile(key string, reader io.Reader, size int64) (string, error) {
	log.Printf("Uploading to S3: bucket=%s, key=%s, size=%d", s.bucketName, key, size)

	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ACL:           types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Return public URL
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucketName, s.region, key)
	log.Printf("Upload successful: %s", url)
	return url, nil
}

// DeleteObject deletes an object from S3
func (s *S3Client) DeleteObject(url string) error {
	// Extract key from URL
	key := s.extractKeyFromURL(url)
	if key == "" {
		log.Printf("Warning: could not extract key from URL: %s", url)
		return nil // Don't fail the request if URL is invalid
	}

	log.Printf("Deleting from S3: bucket=%s, key=%s", s.bucketName, key)

	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	log.Printf("Delete successful: %s", key)
	return nil
}

// extractKeyFromURL extracts the S3 key from a URL
// Expected format: https://bucket.s3.region.amazonaws.com/key
func (s *S3Client) extractKeyFromURL(url string) string {
	// Remove https://
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split by /
	parts := strings.SplitN(url, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}
