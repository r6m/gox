package blobx

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeClient struct {
	data map[string]string
	err  error
}

func (f *fakeClient) PutObject(
	_ context.Context,
	input *s3.PutObjectInput,
	_ ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	f.data[aws.ToString(input.Key)] = string(data)
	return &s3.PutObjectOutput{ETag: aws.String(`"etag"`)}, nil
}

func (f *fakeClient) GetObject(
	_ context.Context,
	input *s3.GetObjectInput,
	_ ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.data[aws.ToString(input.Key)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"}
	}
	size := int64(len(data))
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader(data)),
		ContentLength: &size,
	}, nil
}

func (f *fakeClient) HeadObject(
	_ context.Context,
	input *s3.HeadObjectInput,
	_ ...func(*s3.Options),
) (*s3.HeadObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.data[aws.ToString(input.Key)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NotFound", Message: "missing"}
	}
	size := int64(len(data))
	return &s3.HeadObjectOutput{ContentLength: &size}, nil
}

func (f *fakeClient) DeleteObject(
	_ context.Context,
	input *s3.DeleteObjectInput,
	_ ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	delete(f.data, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func TestStore(t *testing.T) {
	client := &fakeClient{data: make(map[string]string)}
	store, err := newS3Store(client, nil, S3Options{Bucket: "bucket", KeyPrefix: "prefix"})
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Put(context.Background(), "object.txt", strings.NewReader("data"), PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if object.Size != 4 || client.data["prefix/object.txt"] != "data" {
		t.Fatalf("unexpected object: %#v %#v", object, client.data)
	}
}

func TestErrors(t *testing.T) {
	store, err := newS3Store(&fakeClient{data: make(map[string]string)}, nil, S3Options{Bucket: "bucket"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unexpected missing error: %v", err)
	}
	providerErr := errors.New("provider failed")
	store.client = &fakeClient{err: providerErr}
	if _, err := store.Stat(context.Background(), "object"); !errors.Is(err, providerErr) {
		t.Fatalf("provider error was not preserved: %v", err)
	}
}

func TestOptions(t *testing.T) {
	client := &fakeClient{}
	if _, err := newS3Store(client, nil, S3Options{}); err == nil {
		t.Fatal("missing bucket accepted")
	}
	if _, err := newS3Store(client, nil, S3Options{Bucket: "bucket", KeyPrefix: "../bad"}); err == nil {
		t.Fatal("invalid prefix accepted")
	}
}
