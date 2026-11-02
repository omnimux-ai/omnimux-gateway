package tiktok

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("https://www.tiktok.com/v2/auth/authorize/", "ck123", "https://omnimux.ai/api/social/v1/oauth/tiktok/callback", "user.info.basic,video.upload", "state-abc")
	assert.Contains(t, u, "client_key=ck123")
	assert.Contains(t, u, "response_type=code")
	assert.Contains(t, u, "scope=user.info.basic%2Cvideo.upload")
	assert.Contains(t, u, "state=state-abc")
	assert.True(t, strings.HasPrefix(u, "https://www.tiktok.com/v2/auth/authorize/?"))
}

func TestExchangeTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/oauth/token/", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "code123", r.Form.Get("code"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"access_token":"at-1","refresh_token":"rt-1","expires_in":86400,"open_id":"openid-1","scope":"user.info.basic,video.upload"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.ExchangeToken(context.Background(), "ck", "cs", "code123", "https://cb")
	require.NoError(t, err)
	assert.Equal(t, "at-1", tk.AccessToken)
	assert.Equal(t, "rt-1", tk.RefreshToken)
	assert.Equal(t, "openid-1", tk.OpenID)
}

func TestExchangeTokenTopLevelShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-top","refresh_token":"rt-top","expires_in":100,"open_id":"oid-top"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.ExchangeToken(context.Background(), "ck", "cs", "code", "https://cb")
	require.NoError(t, err)
	assert.Equal(t, "at-top", tk.AccessToken)
}

func TestExchangeTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_grant","message":"bad code"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.ExchangeToken(context.Background(), "ck", "cs", "code", "https://cb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "rt-old", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"access_token":"at-new","refresh_token":"rt-new","expires_in":100,"open_id":"oid"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.RefreshToken(context.Background(), "ck", "cs", "rt-old")
	require.NoError(t, err)
	assert.Equal(t, "at-new", tk.AccessToken)
}

func TestGetUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		// Model a token granted only user.info.basic: extended fields fail OAuth binding.
		for _, field := range strings.Split(r.URL.Query().Get("fields"), ",") {
			switch field {
			case "open_id", "union_id", "avatar_url", "display_name":
			default:
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"code":"scope_not_authorized","message":"requires extended profile scope"}}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-1","display_name":"Test User"}}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	u, err := c.GetUserInfo(context.Background(), "at-1")
	require.NoError(t, err)
	assert.Equal(t, "oid-1", u.OpenID)
	assert.Equal(t, "Test User", u.DisplayName)
}

func TestGetUserInfoHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"access_token_invalid","message":"expired"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.GetUserInfo(context.Background(), "bad")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.Status)
	assert.Equal(t, "access_token_invalid", apiErr.Code)
}

func TestRevoke(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotForm = r.Form
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	require.NoError(t, c.Revoke(context.Background(), "ck", "cs", "at-1"))
	assert.Equal(t, "at-1", gotForm.Get("token"))
}

func TestPublishVideoByURL(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/post/publish/video/init/", r.URL.Path)
		assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-1"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	res, err := c.PublishVideoByURL(context.Background(), "at-1", "hello", "https://cdn.example.com/v.mp4", "")
	require.NoError(t, err)
	assert.Equal(t, "pub-1", res.PublishID)
	assert.Contains(t, gotBody, `"privacy_level":"SELF_ONLY"`)
	assert.Contains(t, gotBody, `"video_url":"https://cdn.example.com/v.mp4"`)
}

func TestPublishWithOptionsMapsComplianceFields(t *testing.T) {
	var gotVideoBody, gotPhotoBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		if r.URL.Path == "/v2/post/publish/video/init/" {
			gotVideoBody = string(buf[:n])
		} else {
			gotPhotoBody = string(buf[:n])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-opt"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	f := false
	tr := true
	opts := &PostOptions{
		PrivacyLevel:       "PUBLIC_TO_EVERYONE",
		DisableComment:     &f,
		DisableDuet:        &tr,
		DisableStitch:      &tr,
		BrandContentToggle: &tr,
		BrandOrganicToggle: &f,
	}
	_, err := c.PublishVideoByURLWithOptions(context.Background(), "at-1", "v title", "https://omnimux.ai/v.mp4", opts)
	require.NoError(t, err)
	assert.Contains(t, gotVideoBody, `"privacy_level":"PUBLIC_TO_EVERYONE"`)
	assert.Contains(t, gotVideoBody, `"disable_comment":false`)
	assert.Contains(t, gotVideoBody, `"disable_duet":true`)
	assert.Contains(t, gotVideoBody, `"disable_stitch":true`)
	assert.Contains(t, gotVideoBody, `"brand_content_toggle":true`)
	assert.Contains(t, gotVideoBody, `"brand_organic_toggle":false`)

	_, err = c.PublishPhotoByURLWithOptions(context.Background(), "at-1", "p title", []string{"https://omnimux.ai/p.jpg"}, opts)
	require.NoError(t, err)
	assert.Contains(t, gotPhotoBody, `"disable_comment":false`)
	assert.NotContains(t, gotPhotoBody, `"disable_duet"`, "photo posts must not send duet/stitch flags")
	assert.NotContains(t, gotPhotoBody, `"disable_stitch"`)
}

func TestGetPublishStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":"PUBLISH_COMPLETE","publish_id":"pub-1"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	st, err := c.GetPublishStatus(context.Background(), "at-1", "pub-1")
	require.NoError(t, err)
	assert.Equal(t, "PUBLISH_COMPLETE", st.Status)
}

func TestQueryCreatorInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"creator_nickname":"n","creator_username":"u","privacy_level_options":["SELF_ONLY","FOLLOWER_OF_CREATOR"],"comment_disabled":false,"duet_disabled":true,"stitch_disabled":true,"max_video_post_duration_sec":600}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	info, err := c.QueryCreatorInfo(context.Background(), "at-1")
	require.NoError(t, err)
	assert.Equal(t, "u", info.CreatorUsername)
	assert.False(t, info.CommentDisabled)
	assert.True(t, info.DuetDisabled)
	assert.True(t, info.StitchDisabled)
}
