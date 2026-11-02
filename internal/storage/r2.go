package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// ObjectStore abstracts the S3-compatible API of Cloudflare R2.
type ObjectStore interface {
	PresignPut(ctx context.Context, bucket string, key string, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, bucket string, key string, ttl time.Duration) (string, error)
	Head(ctx context.Context, bucket string, key string) (ObjectInfo, error)
	Get(ctx context.Context, bucket string, key string, maxBytes int64) ([]byte, error)
	Copy(ctx context.Context, sourceBucket string, destBucket string, key string) error
	Delete(ctx context.Context, bucket string, key string) error
}

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type R2Settings struct {
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
	Region          string
}

type R2Store struct {
	client  *s3.Client
	presign *s3.PresignClient
}

func NewR2Store(settings R2Settings) *R2Store {
	region := settings.Region
	if region == "" {
		region = "auto"
	}

	client := s3.New(s3.Options{
		Region:                     region,
		Credentials:                credentials.NewStaticCredentialsProvider(settings.AccessKeyID, settings.SecretAccessKey, ""),
		BaseEndpoint:               aws.String(settings.Endpoint),
		UsePathStyle:               true,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})

	return &R2Store{client: client, presign: s3.NewPresignClient(client)}
}

func (s *R2Store) PresignPut(ctx context.Context, bucket string, key string, contentType string, ttl time.Duration) (string, error) {
	request, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (s *R2Store) PresignGet(ctx context.Context, bucket string, key string, ttl time.Duration) (string, error) {
	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (s *R2Store) Head(ctx context.Context, bucket string, key string) (ObjectInfo, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, err
	}

	return ObjectInfo{Size: aws.ToInt64(output.ContentLength), ContentType: aws.ToString(output.ContentType)}, nil
}

func (s *R2Store) Get(ctx context.Context, bucket string, key string, maxBytes int64) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	defer output.Body.Close()

	data, err := io.ReadAll(io.LimitReader(output.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object exceeds %d bytes", maxBytes)
	}

	return data, nil
}

func (s *R2Store) Copy(ctx context.Context, sourceBucket string, destBucket string, key string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(key),
		CopySource: aws.String(sourceBucket + "/" + key),
	})
	return err
}

func (s *R2Store) Delete(ctx context.Context, bucket string, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return true
		}
	}
	return false
}

type unavailableStore struct{}

func (unavailableStore) PresignPut(context.Context, string, string, string, time.Duration) (string, error) {
	return "", ErrUnavailable
}
func (unavailableStore) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	return "", ErrUnavailable
}
func (unavailableStore) Head(context.Context, string, string) (ObjectInfo, error) {
	return ObjectInfo{}, ErrUnavailable
}
func (unavailableStore) Get(context.Context, string, string, int64) ([]byte, error) {
	return nil, ErrUnavailable
}
func (unavailableStore) Copy(context.Context, string, string, string) error { return ErrUnavailable }
func (unavailableStore) Delete(context.Context, string, string) error       { return ErrUnavailable }

// NewUnavailableStore is used when R2 is not configured so endpoints answer 503 instead of crashing.
func NewUnavailableStore() ObjectStore { return unavailableStore{} }
