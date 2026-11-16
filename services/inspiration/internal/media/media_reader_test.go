package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"omnimux-inspiration/internal/config"
)

func mediaReaderConfig() *config.Config {
	return &config.Config{
		Token:            "fixture",
		CoverR2Endpoint:  "https://fixture.r2.cloudflarestorage.com",
		CoverR2Bucket:    "source-published",
		CoverR2AccessKey: "reader",
		CoverR2SecretKey: "secret",
		R2Endpoint:       "https://upload.r2.cloudflarestorage.com",
		R2Bucket:         "upload-bucket",
		R2AccessKey:      "writer",
		R2SecretKey:      "writer-secret",
		R2Prefix:         "inspirations",
	}
}

const testMediaRef = "r2/publications/genviral/slideshows/example/slide-1.jpg"

func TestMediaReaderUsesOriginalR2Connection(t *testing.T) {
	reader, err := NewMediaReader(mediaReaderConfig())
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("0123456789")
	reader.client.http.Transport = coverRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/source-published/publications/genviral/slideshows/example/slide-1.jpg" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=reader/") {
			t.Fatal("media reader used the upload credentials")
		}
		if r.Header.Get("Range") != "bytes=2-4" {
			t.Fatalf("range=%q", r.Header.Get("Range"))
		}
		return &http.Response{StatusCode: 206, Header: http.Header{
			"Content-Type": {"image/jpeg"}, "Content-Range": {"bytes 2-4/10"}, "Accept-Ranges": {"bytes"}, "Etag": {`"same"`},
		}, Body: io.NopCloser(bytes.NewReader(body[2:5])), ContentLength: 3}, nil
	})
	stream, err := reader.Open(context.Background(), testMediaRef, "bytes=2-4", "")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	got, _ := io.ReadAll(stream.Body)
	if stream.Status != 206 || !bytes.Equal(got, []byte("234")) || stream.ContentRange != "bytes 2-4/10" || stream.ContentType != "image/jpeg" || stream.ETag != `"same"` {
		t.Fatalf("stream=%+v body=%q", stream, got)
	}
}

func TestMediaReaderMapsMissingAndFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
		want   error
	}{
		{"missing", 404, nil, ErrMediaNotFound},
		{"denied", 403, nil, ErrMediaUnavailable},
		{"redirect", 302, nil, ErrMediaUnavailable},
		{"server", 500, nil, ErrMediaUnavailable},
		{"timeout", 0, context.DeadlineExceeded, ErrMediaTimeout},
		{"network", 0, errors.New("network"), ErrMediaUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewMediaReader(mediaReaderConfig())
			if err != nil {
				t.Fatal(err)
			}
			reader.client.http.Transport = coverRoundTrip(func(*http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), ContentLength: 0}, nil
			})
			_, err = reader.Open(context.Background(), testMediaRef, "", "")
			if err != tt.want {
				t.Fatalf("got %v want %v", err, tt.want)
			}
		})
	}
}

func TestMediaReaderRejectsInvalidReferencesAndUnconfiguredR2(t *testing.T) {
	reader, err := NewMediaReader(mediaReaderConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"", "/publications/x.jpg", "r2/../x.jpg", "r2/publications/../x.jpg", "r2/publications//x.jpg", "r2/publications/x?y.jpg"} {
		if _, err := reader.Open(context.Background(), ref, "", ""); err != ErrMediaInvalid {
			t.Fatalf("ref %q: %v", ref, err)
		}
	}
	empty, err := NewMediaReader(&config.Config{})
	if err != nil || empty != nil {
		t.Fatalf("unconfigured reader: %v %v", empty, err)
	}
}
