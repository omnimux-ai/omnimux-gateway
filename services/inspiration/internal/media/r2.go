package media

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"omnimux-inspiration/internal/config"
)

// Minimal S3-compatible Put/Get for Cloudflare R2 (SigV4). No AWS SDK.
type r2Client struct {
	endpoint  string
	accessKey string
	secretKey string
	bucket    string
	region    string
	prefix    string
	http      *http.Client
}

func newR2Client(cfg *config.Config) *r2Client {
	if cfg == nil {
		return nil
	}
	c := &r2Client{
		endpoint:  strings.TrimRight(strings.TrimSpace(cfg.R2Endpoint), "/"),
		accessKey: strings.TrimSpace(cfg.R2AccessKey),
		secretKey: strings.TrimSpace(cfg.R2SecretKey),
		bucket:    strings.TrimSpace(cfg.R2Bucket),
		region:    strings.TrimSpace(cfg.R2Region),
		prefix:    strings.Trim(strings.TrimSpace(cfg.R2Prefix), "/"),
		http:      &http.Client{Timeout: 30 * time.Second},
	}
	if c.region == "" {
		c.region = "auto"
	}
	if !c.configured() {
		return nil
	}
	return c
}

func (c *r2Client) configured() bool {
	return c != nil && c.endpoint != "" && c.accessKey != "" && c.secretKey != "" && c.bucket != ""
}

func (c *r2Client) objectKey(key string) string {
	if c.prefix == "" {
		return key
	}
	return c.prefix + "/" + key
}

func (c *r2Client) Put(ctx context.Context, key string, body []byte, contentType string) error {
	object := c.objectKey(key)
	u := c.endpoint + "/" + c.bucket + "/" + object
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(body))
	if err := c.sign(req, body); err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("r2 put %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

// getResponse shares signing while preserving status and metadata for private covers.
func (c *r2Client) getResponse(ctx context.Context, key string) (*http.Response, error) {
	return c.getResponseWithHeaders(ctx, key, nil)
}

// getResponseWithHeaders keeps Range/If-Range in the signed request. R2 checks
// those headers as part of the signature for range reads.
func (c *r2Client) getResponseWithHeaders(ctx context.Context, key string, headers map[string]string) (*http.Response, error) {
	object := c.objectKey(key)
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = "/" + c.bucket + "/" + object
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		if value != "" {
			req.Header.Set(name, value)
		}
	}
	if err := c.sign(req, nil); err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func (c *r2Client) sign(req *http.Request, payload []byte) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if req.Header.Get("Host") == "" {
		req.Host = req.URL.Host
		req.Header.Set("Host", req.URL.Host)
	}

	signedHeaders, canonicalHeaders := canonicalHeadersOf(req)
	canonicalQuery := canonicalQuery(req.URL.Query())
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := dateStamp + "/" + c.region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signingKey := deriveSigningKey(c.secretKey, dateStamp, c.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, scope, signedHeaders, signature,
	))
	return nil
}

func canonicalHeadersOf(req *http.Request) (signed string, canonical string) {
	keys := make([]string, 0, len(req.Header)+1)
	seen := map[string]bool{}
	for k := range req.Header {
		lk := strings.ToLower(k)
		if lk == "authorization" {
			continue
		}
		if !seen[lk] {
			keys = append(keys, lk)
			seen[lk] = true
		}
	}
	if !seen["host"] {
		keys = append(keys, "host")
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		var v string
		if k == "host" {
			v = req.Host
			if v == "" {
				v = req.URL.Host
			}
		} else {
			v = strings.TrimSpace(strings.Join(req.Header.Values(http.CanonicalHeaderKey(k)), ","))
		}
		v = strings.Join(strings.Fields(v), " ")
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return strings.Join(keys, ";"), b.String()
}

func canonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return values.Encode()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
