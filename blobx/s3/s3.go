// Package s3 provides an AWS SDK v2 S3-compatible blobx store.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/r6m/gox/blobx"
)

type client interface {
	PutObject(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	DeleteObject(context.Context, *awss3.DeleteObjectInput, ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
}

type presignClient interface {
	PresignGetObject(
		context.Context,
		*awss3.GetObjectInput,
		...func(*awss3.PresignOptions),
	) (*v4.PresignedHTTPRequest, error)
}

// Options configures an S3-backed store.
type Options struct {
	Bucket    string
	KeyPrefix string
	Presigner *awss3.PresignClient
}

// Store stores objects in an S3-compatible bucket.
type Store struct {
	client    client
	presigner presignClient
	bucket    string
	prefix    string
}

// New creates an S3-backed store.
func New(client *awss3.Client, opts Options) (*Store, error) {
	if client == nil {
		return nil, errors.New("blobx/s3: client is required")
	}
	var presigner presignClient
	if opts.Presigner != nil {
		presigner = opts.Presigner
	} else {
		presigner = awss3.NewPresignClient(client)
	}
	return newStore(client, presigner, opts)
}

func newStore(client client, presigner presignClient, opts Options) (*Store, error) {
	if client == nil {
		return nil, errors.New("blobx/s3: client is required")
	}
	if opts.Bucket == "" {
		return nil, errors.New("blobx/s3: bucket is required")
	}
	prefix := strings.Trim(opts.KeyPrefix, "/")
	if prefix != "" {
		if _, err := blobx.NormalizeKey(prefix); err != nil {
			return nil, fmt.Errorf("blobx/s3: invalid key prefix: %w", err)
		}
	}
	return &Store{
		client:    client,
		presigner: presigner,
		bucket:    opts.Bucket,
		prefix:    prefix,
	}, nil
}

// Put streams body to S3, replacing any existing object.
func (s *Store) Put(
	ctx context.Context,
	key string,
	body io.Reader,
	opts blobx.PutOptions,
) (blobx.Object, error) {
	key, err := blobx.NormalizeKey(key)
	if err != nil {
		return blobx.Object{}, err
	}
	if body == nil {
		return blobx.Object{}, errors.New("blobx/s3: body is nil")
	}
	input := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.providerKey(key)),
		Body:   body,
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	output, err := s.client.PutObject(ctx, input)
	if err != nil {
		return blobx.Object{}, fmt.Errorf("blobx/s3: put %q: %w", key, err)
	}
	object, err := s.Stat(ctx, key)
	if err != nil {
		return blobx.Object{}, err
	}
	if output.ETag != nil {
		object.ETag = trimETag(*output.ETag)
	}
	return object, nil
}

// Get opens an S3 object for streaming. The caller must close the body.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, blobx.Object, error) {
	key, err := blobx.NormalizeKey(key)
	if err != nil {
		return nil, blobx.Object{}, err
	}
	output, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.providerKey(key)),
	})
	if err != nil {
		return nil, blobx.Object{}, mapError("get", key, err)
	}
	return output.Body, blobx.Object{
		Key:          key,
		Size:         aws.ToInt64(output.ContentLength),
		ContentType:  aws.ToString(output.ContentType),
		ETag:         trimETag(aws.ToString(output.ETag)),
		LastModified: aws.ToTime(output.LastModified),
	}, nil
}

// Stat returns S3 object metadata.
func (s *Store) Stat(ctx context.Context, key string) (blobx.Object, error) {
	key, err := blobx.NormalizeKey(key)
	if err != nil {
		return blobx.Object{}, err
	}
	output, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.providerKey(key)),
	})
	if err != nil {
		return blobx.Object{}, mapError("stat", key, err)
	}
	return blobx.Object{
		Key:          key,
		Size:         aws.ToInt64(output.ContentLength),
		ContentType:  aws.ToString(output.ContentType),
		ETag:         trimETag(aws.ToString(output.ETag)),
		LastModified: aws.ToTime(output.LastModified),
	}, nil
}

// Delete removes an S3 object. S3 deletion is idempotent.
func (s *Store) Delete(ctx context.Context, key string) error {
	key, err := blobx.NormalizeKey(key)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.providerKey(key)),
	})
	if err != nil {
		return fmt.Errorf("blobx/s3: delete %q: %w", key, err)
	}
	return nil
}

// PresignGet creates a temporary URL for reading an object.
func (s *Store) PresignGet(
	ctx context.Context,
	key string,
	opts blobx.PresignOptions,
) (string, error) {
	key, err := blobx.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	if opts.Expires <= 0 {
		return "", errors.New("blobx/s3: presign expiry must be positive")
	}
	output, err := s.presigner.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.providerKey(key)),
	}, func(options *awss3.PresignOptions) {
		options.Expires = opts.Expires
	})
	if err != nil {
		return "", fmt.Errorf("blobx/s3: presign %q: %w", key, err)
	}
	return output.URL, nil
}

func (s *Store) providerKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

func mapError(operation, key string, err error) error {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchObject":
			return fmt.Errorf("blobx/s3: %s %q: %w: %v", operation, key, blobx.ErrNotFound, err)
		}
	}
	return fmt.Errorf("blobx/s3: %s %q: %w", operation, key, err)
}

func trimETag(value string) string {
	return strings.Trim(value, `"`)
}

var _ blobx.Store = (*Store)(nil)
var _ blobx.Presigner = (*Store)(nil)
