package api

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"omnimux-inspiration/internal/media"
)

type fakeMediaStreamer struct {
	stream *media.MediaStream
	err    error
	key    string
	rangeH string
	ifRng  string
}

func (f *fakeMediaStreamer) Open(_ context.Context, key, rangeHeader, ifRange string) (*media.MediaStream, error) {
	f.key, f.rangeH, f.ifRng = key, rangeHeader, ifRange
	return f.stream, f.err
}

func TestStreamMediaResponses(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	tests := []struct {
		name   string
		stream *media.MediaStream
		err    error
		status int
		body   string
		header map[string]string
	}{
		{"full", &media.MediaStream{Body: io.NopCloser(bytes.NewReader([]byte("0123456789"))), Status: 200, ContentType: "image/jpeg", ContentLength: "10", ETag: `"v1"`}, nil, 200, "0123456789", map[string]string{"Content-Type": "image/jpeg", "ETag": `"v1"`}},
		{"range", &media.MediaStream{Body: io.NopCloser(bytes.NewReader([]byte("234"))), Status: 206, ContentType: "image/jpeg", ContentRange: "bytes 2-4/10", AcceptRanges: "bytes"}, nil, 206, "234", map[string]string{"Content-Range": "bytes 2-4/10", "Accept-Ranges": "bytes"}},
		{"missing", nil, media.ErrMediaNotFound, 404, "", nil},
		{"unavailable", nil, media.ErrMediaUnavailable, 503, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeMediaStreamer{stream: tt.stream, err: tt.err}
			h := &Handler{MediaStreamer: fake}
			r := gin.New()
			r.GET("/media/*key", func(c *gin.Context) { h.StreamMedia(c, c.Param("key")[1:]) })
			req := httptest.NewRequest("GET", "/media/r2/publications/a/slide-1.jpg", nil)
			req.Header.Set("Range", "bytes=0-0")
			req.Header.Set("If-Range", `"v1"`)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status || tt.body != "" && w.Body.String() != tt.body {
				t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
			}
			for k, v := range tt.header {
				if w.Header().Get(k) != v {
					t.Fatalf("%s=%q want %q", k, w.Header().Get(k), v)
				}
			}
			if tt.status == 200 || tt.status == 206 {
				if fake.key != "r2/publications/a/slide-1.jpg" || fake.rangeH != "bytes=0-0" || fake.ifRng != `"v1"` {
					t.Fatalf("forwarded key=%q range=%q ifRange=%q", fake.key, fake.rangeH, fake.ifRng)
				}
			}
		})
	}
}
