package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeS3 struct {
	input *s3.PutObjectInput
	err   error
}

func (f *fakeS3) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.input = params
	if f.err != nil {
		return nil, f.err
	}
	return &s3.PutObjectOutput{}, nil
}

func TestS3StorePutJSON(t *testing.T) {
	fake := &fakeS3{}
	store := &S3Store{client: fake, bucket: "prasad-cloud-demo"}
	body := []byte(`{"id":"a83f21"}`)

	err := store.PutJSON(context.Background(), "submissions/example.json", body)
	if err != nil {
		t.Fatal(err)
	}
	if fake.input == nil {
		t.Fatal("PutObject was not called")
	}
	if aws.ToString(fake.input.Bucket) != "prasad-cloud-demo" {
		t.Fatalf("bucket = %q", aws.ToString(fake.input.Bucket))
	}
	if aws.ToString(fake.input.Key) != "submissions/example.json" {
		t.Fatalf("key = %q", aws.ToString(fake.input.Key))
	}
	if aws.ToString(fake.input.ContentType) != "application/json" {
		t.Fatalf("content type = %q", aws.ToString(fake.input.ContentType))
	}
	got, err := io.ReadAll(fake.input.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body = %s", got)
	}
}

func TestS3StorePutJSONError(t *testing.T) {
	fake := &fakeS3{err: errors.New("access denied")}
	store := &S3Store{client: fake, bucket: "prasad-cloud-demo"}
	err := store.PutJSON(context.Background(), "submissions/example.json", []byte("{}"))
	if err == nil {
		t.Fatal("expected an error")
	}
}
