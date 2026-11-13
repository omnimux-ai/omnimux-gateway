package tiktok

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TikTok v2 includes an error object even on successful responses.
func TestV2SuccessEnvelope(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, data string
		call       func(*Client) error
	}{
		{"user", `{"user":{"open_id":"oid","display_name":"Tester"}}`, func(c *Client) error {
			v, e := c.GetUserInfo(ctx, "test")
			if e == nil {
				assert.Equal(t, "oid", v.OpenID)
			}
			return e
		}},
		{"creator", `{"creator_username":"tester"}`, func(c *Client) error { _, e := c.QueryCreatorInfo(ctx, "test"); return e }},
		{"video", `{"publish_id":"pub"}`, func(c *Client) error {
			_, e := c.PublishVideoByURL(ctx, "test", "title", "https://example.com/video.mp4", "SELF_ONLY")
			return e
		}},
		{"photo", `{"publish_id":"pub"}`, func(c *Client) error {
			_, e := c.PublishPhotoByURL(ctx, "test", "title", []string{"https://example.com/photo.jpg"}, "SELF_ONLY")
			return e
		}},
		{"status", `{"status":"PUBLISH_COMPLETE"}`, func(c *Client) error { _, e := c.GetPublishStatus(ctx, "test", "pub"); return e }},
		{"upload_init", `{"publish_id":"pub"}`, func(c *Client) error {
			_, e := c.UploadVideoFile(ctx, "test", "title", []byte("test"), "video.mp4", "SELF_ONLY")
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, code := range []string{"ok", "scope_not_authorized", ""} {
				t.Run(code, func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"data":` + tc.data + `,"error":{"code":"` + code + `","message":"","log_id":"fixture"}}`))
					}))
					defer srv.Close()
					err := tc.call(NewClient(srv.URL))
					if code == "ok" {
						require.NoError(t, err)
					} else {
						var apiErr *APIError
						require.ErrorAs(t, err, &apiErr)
						assert.Equal(t, code, apiErr.Code)
					}
				})
			}
		})
	}
}

func TestTokenOKWithoutDataStillFails(t *testing.T) {
	_, err := parseToken([]byte(`{"error":{"code":"ok"}}`))
	require.ErrorContains(t, err, "empty access_token")
}
