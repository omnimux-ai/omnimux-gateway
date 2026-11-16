package media

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/errs"
)

var (
	ErrMediaNotFound    = errs.Wrap("MEDIA_NOT_FOUND", http.StatusNotFound, "media not found", nil)
	ErrMediaInvalid     = errs.Wrap("INVALID_PARAM", http.StatusBadRequest, "invalid media key", nil)
	ErrMediaUnavailable = errs.Wrap("MEDIA_UNAVAILABLE", http.StatusServiceUnavailable, "media storage unavailable", nil)
	ErrMediaTimeout     = errs.Wrap("MEDIA_TIMEOUT", http.StatusGatewayTimeout, "media storage timeout", nil)
)

// MediaStream is a streaming response from the original-media R2 bucket.
// The caller owns Body and must close it after writing the response.
type MediaStream struct {
	Body          io.ReadCloser
	Status        int
	ContentType   string
	ContentLength string
	ContentRange  string
	AcceptRanges  string
	ETag          string
	LastModified  string
	CacheControl  string
}

// MediaReader serves r2/publications/... objects from the same read-only R2
// bucket as original covers. Media keys are object keys, not local paths: the
// sidecar's local volume only contains seed/backfill files, so the route must
// fall through here when no local copy exists.
type MediaReader struct {
	client *r2Client
}

// NewMediaReader uses the dedicated original-cover connection. It deliberately
// does not fall back to the upload R2 configuration or its `inspirations`
// prefix: those are separate buckets with separate credentials.
func NewMediaReader(cfg *config.Config) (*MediaReader, error) {
	if err := cfg.ValidateCoverR2(); err != nil {
		return nil, err
	}
	if cfg == nil || cfg.CoverR2Endpoint == "" {
		return nil, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 15 * time.Second
	return &MediaReader{client: &r2Client{
		endpoint:  cfg.CoverR2Endpoint,
		bucket:    cfg.CoverR2Bucket,
		accessKey: cfg.CoverR2AccessKey,
		secretKey: cfg.CoverR2SecretKey,
		region:    "auto",
		http: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}}, nil
}

// Open starts a GET for ref, forwarding Range/If-Range to R2. A 404 is a
// missing object, not a storage outage; 5xx and network failures are retryable
// service errors so the gateway can distinguish them for clients.
func (r *MediaReader) Open(ctx context.Context, ref, rangeHeader, ifRangeHeader string) (*MediaStream, error) {
	if r == nil || r.client == nil {
		return nil, ErrMediaUnavailable
	}
	key, err := ParseCoverRef(ref)
	if err != nil {
		return nil, ErrMediaInvalid
	}
	headers := map[string]string{}
	if rangeHeader != "" {
		headers["Range"] = rangeHeader
	}
	if ifRangeHeader != "" {
		headers["If-Range"] = ifRangeHeader
	}
	resp, err := r.client.getResponseWithHeaders(ctx, key, headers)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrMediaTimeout
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, ErrMediaTimeout
		}
		return nil, ErrMediaUnavailable
	}
	status := resp.StatusCode
	if status != http.StatusOK && status != http.StatusPartialContent && status != http.StatusRequestedRangeNotSatisfiable {
		defer resp.Body.Close()
		if status == http.StatusNotFound {
			return nil, ErrMediaNotFound
		}
		return nil, ErrMediaUnavailable
	}
	return &MediaStream{
		Body:          resp.Body,
		Status:        status,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.Header.Get("Content-Length"),
		ContentRange:  resp.Header.Get("Content-Range"),
		AcceptRanges:  resp.Header.Get("Accept-Ranges"),
		ETag:          resp.Header.Get("ETag"),
		LastModified:  resp.Header.Get("Last-Modified"),
		CacheControl:  resp.Header.Get("Cache-Control"),
	}, nil
}
