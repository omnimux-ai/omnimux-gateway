package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// R2StorageDriver implements StorageDriver using Cloudflare R2 via AWS S3 SDK.
type R2StorageDriver struct {
	client        *s3.Client
	presign       *s3.PresignClient
	bucket        string
	publicBaseURL string
}

// NewR2StorageDriver creates an R2 storage driver with explicit credentials.
func NewR2StorageDriver(endpoint, bucket, accessKey, secretKey, publicBaseURL string) (*R2StorageDriver, error) {
	if bucket == "" {
		return nil, fmt.Errorf("R2 bucket cannot be empty")
	}
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("R2 access key and secret key cannot be empty")
	}
	parsedEndpoint, endpointErr := url.Parse(endpoint)
	if endpointErr != nil || parsedEndpoint.Hostname() == "" || parsedEndpoint.User != nil || (parsedEndpoint.Scheme != "https" && parsedEndpoint.Scheme != "http") || parsedEndpoint.RawQuery != "" || parsedEndpoint.Fragment != "" {
		return nil, fmt.Errorf("R2 endpoint must be an HTTP(S) origin without credentials")
	}

	cfg := aws.Config{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// fork: r2-file-storage-presign — a presigned PUT is only reproducible when
	// nothing extra is added to the signature, so the ticket client never
	// attaches the default trailing checksum the upload client still sends.
	ticketClient := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	return &R2StorageDriver{
		client:        client,
		presign:       s3.NewPresignClient(ticketClient),
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

// NewR2StorageDriverFromEnv attempts to initialize R2 storage from environment variables.
func NewR2StorageDriverFromEnv() (*R2StorageDriver, error) {
	bucket := os.Getenv("R2_BUCKET_NAME")
	if bucket == "" {
		bucket = os.Getenv("R2_BUCKET")
	}
	if bucket == "" {
		return nil, fmt.Errorf("R2_BUCKET_NAME not set")
	}

	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("R2_ACCESS_KEY_ID or R2_SECRET_ACCESS_KEY not set")
	}

	endpoint := os.Getenv("R2_ENDPOINT")
	if endpoint == "" {
		accountID := os.Getenv("R2_ACCOUNT_ID")
		if accountID != "" {
			endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
		}
	}
	if endpoint == "" {
		return nil, fmt.Errorf("R2_ENDPOINT or R2_ACCOUNT_ID not set")
	}

	publicBaseURL := os.Getenv("R2_PUBLIC_BASE_URL")
	if publicBaseURL == "" {
		publicBaseURL = os.Getenv("R2_PUBLIC_DOMAIN")
	}

	return NewR2StorageDriver(endpoint, bucket, accessKey, secretKey, publicBaseURL)
}

func (r *R2StorageDriver) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	input := &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(cleanKey),
		Body:   bytes.NewReader(data),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := r.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload object to R2: %w", err)
	}

	return r.GetPublicURL(cleanKey), nil
}

// PresignPut hands out a short-lived PUT ticket for one object key. A configured
// public origin is not a precondition: deployments that serve objects through
// the gateway download route still accept the bytes this way, so the ticket is
// refused only when the backend cannot presign at all.
func (r *R2StorageDriver) PresignPut(ctx context.Context, key string, contentType string, ttl time.Duration) (*PresignResult, error) {
	if r.presign == nil {
		return nil, ErrPresignUnsupported
	}
	if ttl < time.Second || ttl > 7*24*time.Hour {
		return nil, fmt.Errorf("presign ttl must be between one second and seven days")
	}
	cleanKey := strings.TrimPrefix(key, "/")
	if cleanKey == "" {
		return nil, fmt.Errorf("storage key cannot be empty")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	req, err := r.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(cleanKey),
		ContentType: aws.String(contentType),
		IfNoneMatch: aws.String("*"),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return nil, fmt.Errorf("failed to presign object upload: %w", err)
	}

	headers := make(map[string]string, len(req.SignedHeader))
	for name, values := range req.SignedHeader {
		if len(values) == 0 {
			continue
		}
		headers[name] = strings.Join(values, ",")
	}

	return &PresignResult{
		URL:       req.URL,
		Method:    strings.ToUpper(req.Method),
		Headers:   headers,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// Head avoids receiving object data during upload confirmation and artifact resolution.
func (r *R2StorageDriver) Head(ctx context.Context, key string) (*ObjectMetadata, error) {
	output, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(strings.TrimPrefix(key, "/"))})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && (apiError.ErrorCode() == "NotFound" || apiError.ErrorCode() == "NoSuchKey") {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to inspect object in R2: %w", err)
	}
	return &ObjectMetadata{ContentType: aws.ToString(output.ContentType), Size: aws.ToInt64(output.ContentLength)}, nil
}

// PresignGet preserves browser Range requests while binding the final response policy.
func (r *R2StorageDriver) PresignGet(ctx context.Context, key, contentType, disposition string, ttl time.Duration) (*PresignResult, error) {
	if r.presign == nil {
		return nil, ErrPresignUnsupported
	}
	if ttl < time.Second || ttl > 7*24*time.Hour || strings.Trim(key, "/") == "" || strings.ContainsAny(contentType+disposition, "\r\n") {
		return nil, fmt.Errorf("invalid download ticket arguments")
	}
	request, err := r.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket), Key: aws.String(strings.TrimPrefix(key, "/")),
		ResponseContentType: aws.String(contentType), ResponseContentDisposition: aws.String(disposition),
		ResponseCacheControl: aws.String("private, no-store"),
	}, func(options *s3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return nil, fmt.Errorf("failed to sign object download: %w", err)
	}
	return &PresignResult{URL: request.URL, Method: request.Method, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (r *R2StorageDriver) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	output, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get object from R2: %w", err)
	}

	contentType := "application/octet-stream"
	if output.ContentType != nil && *output.ContentType != "" {
		contentType = *output.ContentType
	}
	var size int64
	if output.ContentLength != nil {
		size = *output.ContentLength
	}

	return output.Body, contentType, size, nil
}

func (r *R2StorageDriver) Delete(ctx context.Context, key string) error {
	cleanKey := strings.TrimPrefix(key, "/")
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from R2: %w", err)
	}
	return nil
}

func (r *R2StorageDriver) GetPublicURL(key string) string {
	return publicObjectURL(r.publicBaseURL, key)
}

func (r *R2StorageDriver) IsR2() bool {
	return true
}
