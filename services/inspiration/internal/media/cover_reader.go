package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"time"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/errs"
)

const maxCoverBytes = 5 << 20

var (
	ErrCoverNotFound    = errs.Wrap("COVER_NOT_FOUND", 404, "cover not found", nil)
	ErrCoverUnavailable = errs.Wrap("COVER_STORAGE_UNAVAILABLE", 503, "cover storage unavailable", nil)
	ErrCoverTimeout     = errs.Wrap("COVER_STORAGE_TIMEOUT", 504, "cover storage timeout", nil)
	ErrCoverInvalid     = errs.Wrap("COVER_INVALID", 502, "invalid cover image", nil)
)

type CoverImage struct {
	Data         []byte
	ContentType  string
	ETag         string
	LastModified time.Time
}

// CoverReader owns only a read connection; upload settings cannot select its bucket.
type CoverReader struct{ client *r2Client }

func NewCoverReader(cfg *config.Config) (*CoverReader, error) {
	if err := cfg.ValidateCoverR2(); err != nil {
		return nil, err
	}
	if cfg.CoverR2Endpoint == "" {
		return nil, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &CoverReader{client: &r2Client{
		endpoint: cfg.CoverR2Endpoint, bucket: cfg.CoverR2Bucket,
		accessKey: cfg.CoverR2AccessKey, secretKey: cfg.CoverR2SecretKey, region: "auto",
		http: &http.Client{Timeout: 15 * time.Second, Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}}, nil
}

func (r *CoverReader) Read(ctx context.Context, ref string) (*CoverImage, error) {
	key, err := ParseCoverRef(ref)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrCoverUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := r.client.getResponse(ctx, key)
	if err != nil {
		return nil, coverReadError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrCoverNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrCoverUnavailable
	}
	if resp.ContentLength > maxCoverBytes {
		return nil, ErrCoverInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes+1))
	if err != nil {
		return nil, coverReadError(err)
	}
	if len(raw) > maxCoverBytes {
		return nil, ErrCoverInvalid
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 8 || cfg.Height < 8 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, ErrCoverInvalid
	}
	if _, _, err = image.Decode(bytes.NewReader(raw)); err != nil {
		return nil, ErrCoverInvalid
	}
	types := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "gif": "image/gif"}
	ctype, ok := types[format]
	if !ok {
		return nil, ErrCoverInvalid
	}
	modified, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return &CoverImage{Data: raw, ContentType: ctype, ETag: resp.Header.Get("ETag"), LastModified: modified}, nil
}

func coverReadError(err error) error {
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return ErrCoverTimeout
	}
	return ErrCoverUnavailable
}
