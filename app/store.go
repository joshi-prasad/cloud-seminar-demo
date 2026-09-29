package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectStore saves one JSON document under an object key.
// Handlers depend on this interface so tests can fake S3.
type ObjectStore interface {
	PutJSON(ctx context.Context, key string, body []byte) error
}

type objectPutter interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Store writes objects with the AWS SDK for Go v2.
// The client uses the default credential chain (environment, shared config,
// then the EC2 instance IAM role). This code does not read or log credentials.
type S3Store struct {
	client objectPutter
	bucket string
}

func newS3Store(ctx context.Context, region, bucket string) (*S3Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &S3Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

func (s *S3Store) PutJSON(ctx context.Context, key string, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}
