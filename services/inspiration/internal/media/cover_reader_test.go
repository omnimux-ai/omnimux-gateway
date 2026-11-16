package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnimux-inspiration/internal/config"
)

const testCoverRef = "r2/publications/genviral/videos/example/cover.png"

type coverRoundTrip func(*http.Request) (*http.Response, error)

func (f coverRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func coverPNG(t *testing.T, size int) []byte {
	t.Helper()
	var b bytes.Buffer
	m := image.NewRGBA(image.Rect(0, 0, size, size))
	m.Set(0, 0, color.White)
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func coverConfig() *config.Config {
	return &config.Config{Token: "fixture", CoverR2Endpoint: "https://fixture.r2.cloudflarestorage.com", CoverR2Bucket: "source-published", CoverR2AccessKey: "reader", CoverR2SecretKey: "fixture-only"}
}

func TestCoverReference(t *testing.T) {
	for _, ref := range []string{"", "publications/a.jpg", "r2/publications/../x", "r2/publications/%2e%2e/x", "r2/publications//x", "r2/publications/x?y", "r2/publications/x\\y", "r2/publications/x\x00", "r2/publications/x/", "r2/publications/ x", "r2/https://host/x", "r2/publications/" + strings.Repeat("x", 250)} {
		if _, err := ParseCoverRef(ref); err == nil {
			t.Errorf("accepted %q", ref)
		}
	}
	key, err := ParseCoverRef(testCoverRef)
	if err != nil || key != "publications/genviral/videos/example/cover.png" {
		t.Fatalf("key=%q err=%v", key, err)
	}
}

func TestCoverConfiguration(t *testing.T) {
	empty, err := NewCoverReader(&config.Config{})
	if err != nil || empty != nil {
		t.Fatal(empty, err)
	}
	for _, change := range []func(*config.Config){func(c *config.Config) { c.Token = "" }, func(c *config.Config) { c.CoverR2SecretKey = "" }, func(c *config.Config) { c.CoverR2Endpoint = "http://fixture.r2.cloudflarestorage.com" }, func(c *config.Config) { c.CoverR2Endpoint = "https://evil.example" }, func(c *config.Config) { c.CoverR2Endpoint += "/path" }, func(c *config.Config) { c.CoverR2Bucket = "../bucket" }} {
		cfg := coverConfig()
		change(cfg)
		if _, err := NewCoverReader(cfg); err == nil {
			t.Fatal("accepted invalid config")
		}
	}
}

func TestCoverReadUsesDedicatedConnection(t *testing.T) {
	cfg := coverConfig()
	cfg.R2Bucket = "upload-bucket"
	cfg.R2Prefix = "inspirations"
	cfg.R2AccessKey = "writer"
	reader, err := NewCoverReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw := coverPNG(t, 16)
	reader.client.http.Transport = coverRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/source-published/publications/genviral/videos/example/cover.png" {
			t.Fatal(r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=reader/") {
			t.Fatal("wrong signer")
		}
		if r.Method != "GET" {
			t.Fatal(r.Method)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}, "Etag": {"\"original\""}}, Body: io.NopCloser(bytes.NewReader(raw)), ContentLength: int64(len(raw))}, nil
	})
	got, err := reader.Read(t.Context(), testCoverRef)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, raw) || got.ContentType != "image/png" || got.ETag != "\"original\"" {
		t.Fatal("image metadata mismatch")
	}
}

func TestCoverFailures(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    []byte
		readErr error
		want    error
	}{
		{"missing", 404, nil, nil, ErrCoverNotFound}, {"denied", 403, nil, nil, ErrCoverUnavailable}, {"redirect", 302, nil, nil, ErrCoverUnavailable},
		{"timeout", 0, nil, context.DeadlineExceeded, ErrCoverTimeout}, {"network", 0, nil, errors.New("network"), ErrCoverUnavailable},
		{"html", 200, []byte("<html>failed</html>"), nil, ErrCoverInvalid}, {"tiny", 200, coverPNG(t, 1), nil, ErrCoverInvalid}, {"oversize", 200, make([]byte, maxCoverBytes+1), nil, ErrCoverInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, _ := NewCoverReader(coverConfig())
			reader.client.http.Transport = coverRoundTrip(func(*http.Request) (*http.Response, error) {
				if tt.readErr != nil {
					return nil, tt.readErr
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(tt.body)), ContentLength: int64(len(tt.body))}, nil
			})
			if _, err := reader.Read(t.Context(), testCoverRef); err != tt.want {
				t.Fatalf("got %v want %v", err, tt.want)
			}
		})
	}
	var nilReader *CoverReader
	if _, err := nilReader.Read(t.Context(), testCoverRef); err != ErrCoverUnavailable {
		t.Fatal(err)
	}
}

func TestCoverDoesNotFollowRedirect(t *testing.T) {
	reached := 0
	dest := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++; w.Write(coverPNG(t, 16)) }))
	defer dest.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 302) }))
	defer source.Close()
	reader, _ := NewCoverReader(coverConfig())
	reader.client.endpoint = source.URL
	reader.client.http.Transport = source.Client().Transport
	if _, err := reader.Read(t.Context(), testCoverRef); err != ErrCoverUnavailable {
		t.Fatal(err)
	}
	if reached != 0 {
		t.Fatal("redirect destination reached")
	}
}

func TestCoverUnknownLengthAndTruncatedImage(t *testing.T) {
	raw := coverPNG(t, 16)
	for _, tt := range []struct {
		name string
		body []byte
	}{{"unknown oversized", make([]byte, maxCoverBytes+1)}, {"truncated", raw[:len(raw)/2]}} {
		t.Run(tt.name, func(t *testing.T) {
			reader, _ := NewCoverReader(coverConfig())
			reader.client.http.Transport = coverRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(tt.body)), ContentLength: -1}, nil
			})
			if _, err := reader.Read(t.Context(), testCoverRef); err != ErrCoverInvalid {
				t.Fatalf("got %v", err)
			}
		})
	}
}
